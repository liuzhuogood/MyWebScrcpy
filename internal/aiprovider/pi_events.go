package aiprovider

import (
	"encoding/json"
	"errors"
	"strings"
)

// ChatEvent is the SSE payload shared with the assistant frontend.
type ChatEvent struct {
	Type       string          `json:"type"`
	Delta      string          `json:"delta,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Args       json.RawMessage `json:"args,omitempty"`
	Output     string          `json:"output,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
	Reply      string          `json:"reply,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type piContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type piMessage struct {
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	StopReason   string          `json:"stopReason"`
	ErrorMessage string          `json:"errorMessage"`
}
type piEvent struct {
	Type                  string `json:"type"`
	Command               string `json:"command"`
	Success               bool   `json:"success"`
	Error                 string `json:"error"`
	AssistantMessageEvent struct {
		Type  string `json:"type"`
		Delta string `json:"delta"`
	} `json:"assistantMessageEvent"`
	Message       piMessage       `json:"message"`
	ToolCallID    string          `json:"toolCallId"`
	ToolName      string          `json:"toolName"`
	Args          json.RawMessage `json:"args"`
	PartialResult struct {
		Content []piContent `json:"content"`
	} `json:"partialResult"`
	Result struct {
		Content []piContent `json:"content"`
	} `json:"result"`
	IsError bool `json:"isError"`
}

type piStreamState struct {
	reply        string
	messageError string
}

func (s *piStreamState) consume(raw piEvent, emit func(ChatEvent) error) (bool, error) {
	event := ChatEvent{}
	switch raw.Type {
	case "response":
		if raw.Command == "prompt" && !raw.Success {
			return false, errors.New(raw.Error)
		}
		return false, nil
	case "message_update":
		switch raw.AssistantMessageEvent.Type {
		case "thinking_start", "thinking_end", "text_start", "text_end":
			event.Type = raw.AssistantMessageEvent.Type
		case "thinking_delta", "text_delta":
			event.Type, event.Delta = raw.AssistantMessageEvent.Type, raw.AssistantMessageEvent.Delta
		default:
			return false, nil
		}
	case "message_end":
		if raw.Message.Role == "assistant" {
			if raw.Message.StopReason == "error" || raw.Message.StopReason == "aborted" {
				s.messageError = raw.Message.ErrorMessage
				if s.messageError == "" {
					s.messageError = "Pi 模型调用失败或已中止"
				}
				return false, nil
			}
			var content []piContent
			if err := json.Unmarshal(raw.Message.Content, &content); err != nil {
				return false, err
			}
			s.reply = piText(content)
			s.messageError = ""
		}
		return false, nil
	case "tool_execution_start", "tool_execution_update", "tool_execution_end":
		if raw.ToolCallID == "" || raw.ToolName == "" {
			return false, errors.New("Pi 工具事件缺少 toolCallId 或 toolName")
		}
		event.ToolCallID, event.ToolName = raw.ToolCallID, raw.ToolName
		switch raw.Type {
		case "tool_execution_start":
			event.Type, event.Args = "tool_start", raw.Args
		// Pi sends accumulated output here; clients must replace, not append.
		case "tool_execution_update":
			event.Type, event.Output = "tool_update", piText(raw.PartialResult.Content)
		case "tool_execution_end":
			event.Type, event.Output, event.IsError = "tool_end", piText(raw.Result.Content), raw.IsError
		}
	case "agent_settled":
		if s.messageError != "" {
			return false, errors.New(s.messageError)
		}
		if strings.TrimSpace(s.reply) == "" {
			return false, errors.New("Pi 未返回有效回复")
		}
		return true, emit(ChatEvent{Type: "done", Reply: s.reply})
	default:
		return false, nil
	}
	return false, emit(event)
}

func piText(content []piContent) string {
	var text strings.Builder
	for _, part := range content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}
	return text.String()
}
