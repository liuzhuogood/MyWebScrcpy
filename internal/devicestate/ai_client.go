package devicestate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
)

var (
	ErrAIBusy          = errors.New("AI 决策请求超出最大并发限制 (最多 2 个并发请求)")
	ErrAINotConfigured = errors.New("AI 视觉服务未配置：请在系统设置中配置端点与模型")
)

const (
	ConfigKeyAIEndpoint = "ai_chat_endpoint"
	ConfigKeyAIModel    = "ai_model"
	ConfigKeyAIApiKey   = "ai_api_key"
	ConfigKeyAITimeout  = "ai_timeout_seconds"

	DefaultAITimeoutSeconds = 30
	MaxConcurrentAIRequests = 2
)

// AIClient 封装基于官方视觉协议的 Chat Completions 调用
type AIClient struct {
	database  *db.DB
	client    *http.Client
	semaphore chan struct{}
	mu        sync.Mutex
}

func NewAIClient(database *db.DB) *AIClient {
	return &AIClient{
		database:  database,
		client:    &http.Client{},
		semaphore: make(chan struct{}, MaxConcurrentAIRequests),
	}
}

// GetConfig 读取当前 AI 服务配置（密钥掩码脱敏，检查数据库读取错误）
func (c *AIClient) GetConfig() (ServiceConfig, error) {
	if c.database == nil {
		return ServiceConfig{AITimeoutSeconds: DefaultAITimeoutSeconds}, nil
	}

	endpoint, err := c.database.GetConfig(ConfigKeyAIEndpoint)
	if err != nil {
		return ServiceConfig{}, fmt.Errorf("读取端点配置失败: %w", err)
	}
	model, err := c.database.GetConfig(ConfigKeyAIModel)
	if err != nil {
		return ServiceConfig{}, fmt.Errorf("读取模型配置失败: %w", err)
	}
	rawKey, err := c.database.GetConfig(ConfigKeyAIApiKey)
	if err != nil {
		return ServiceConfig{}, fmt.Errorf("读取密钥配置失败: %w", err)
	}
	timeoutStr, err := c.database.GetConfig(ConfigKeyAITimeout)
	if err != nil {
		return ServiceConfig{}, fmt.Errorf("读取超时配置失败: %w", err)
	}

	timeout := DefaultAITimeoutSeconds
	if timeoutStr != "" {
		var t int
		if _, err := fmt.Sscanf(timeoutStr, "%d", &t); err == nil && t > 0 {
			timeout = t
		}
	}

	maskedKey := ""
	hasKey := false
	if len(rawKey) > 0 {
		hasKey = true
		if len(rawKey) <= 8 {
			maskedKey = "******"
		} else {
			maskedKey = rawKey[:4] + "****" + rawKey[len(rawKey)-4:]
		}
	}

	return ServiceConfig{
		AIChatEndpoint:   endpoint,
		AIModel:          model,
		AIApiKeyMasked:   maskedKey,
		AIHasKey:         hasKey,
		AITimeoutSeconds: timeout,
	}, nil
}

func sanitizeSecret(msg, secret string) string {
	if secret == "" || len(strings.TrimSpace(secret)) < 3 {
		return msg
	}
	s := strings.TrimSpace(secret)
	return strings.ReplaceAll(msg, s, "[REDACTED]")
}

// ValidateConfig 严格验证 AI 配置项的合法性，不进行任何写操作
func (c *AIClient) ValidateConfig(cfg ServiceConfig) error {
	if cfg.AIChatEndpoint != "" {
		ep := strings.TrimSpace(cfg.AIChatEndpoint)
		if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
			return errors.New("AI 端点地址必须以 http:// 或 https:// 开头")
		}
		u, err := url.Parse(ep)
		if err != nil || u.Host == "" {
			return errors.New("AI 端点地址必须为包含主机的合法 URL (如 https://api.openai.com/v1/chat/completions)")
		}
		if u.User != nil {
			return errors.New("AI 端点地址中禁止嵌入认证账号密码")
		}
	}
	if cfg.AIModel != "" {
		m := strings.TrimSpace(cfg.AIModel)
		if len(m) > 128 {
			return errors.New("AI 模型名称长度不能超过 128 字符")
		}
	}
	if cfg.AITimeoutSeconds != 0 {
		if cfg.AITimeoutSeconds < 1 || cfg.AITimeoutSeconds > 30 {
			return errors.New("AI 超时时间必须在 1 到 30 秒之间")
		}
	}
	return nil
}

// SaveConfig 保存 AI 服务配置（严格原子更新与脱敏保护）
func (c *AIClient) SaveConfig(cfg ServiceConfig) error {
	if c.database == nil {
		return errors.New("数据库不可用")
	}

	if err := c.ValidateConfig(cfg); err != nil {
		return err
	}

	// 1. 端点
	if cfg.AIChatEndpoint != "" {
		ep := strings.TrimSpace(cfg.AIChatEndpoint)
		if err := c.database.SetConfig(ConfigKeyAIEndpoint, ep); err != nil {
			return err
		}
	}

	// 2. 模型
	if cfg.AIModel != "" {
		if err := c.database.SetConfig(ConfigKeyAIModel, strings.TrimSpace(cfg.AIModel)); err != nil {
			return err
		}
	}

	// 3. 密钥更新：优先使用显式 AIApiKey；若未传但 AIApiKeyMasked 传入非掩码明文，亦予以保存
	keyToSave := strings.TrimSpace(cfg.AIApiKey)
	if keyToSave == "" && cfg.AIApiKeyMasked != "" && !strings.Contains(cfg.AIApiKeyMasked, "*") {
		keyToSave = strings.TrimSpace(cfg.AIApiKeyMasked)
	}
	if keyToSave != "" {
		if err := c.database.SetConfig(ConfigKeyAIApiKey, keyToSave); err != nil {
			return err
		}
	}

	// 4. 超时
	if cfg.AITimeoutSeconds > 0 {
		if err := c.database.SetConfig(ConfigKeyAITimeout, fmt.Sprintf("%d", cfg.AITimeoutSeconds)); err != nil {
			return err
		}
	}

	return nil
}

// Decide 执行单轮视觉决策，严格绑定 observationID，不触发设备操作
func (c *AIClient) Decide(ctx context.Context, state *DeviceState, requestedGoal string) AIDecisionResult {
	start := time.Now()

	if c == nil || c.database == nil {
		obsID := ""
		if state != nil {
			obsID = state.ObservationID
		}
		return AIDecisionResult{
			Available:     false,
			ObservationID: obsID,
			Error:         "数据库不可用",
			DurationMS:    0,
		}
	}
	if state == nil {
		return AIDecisionResult{
			Available:  false,
			Error:      "未提供设备状态",
			DurationMS: 0,
		}
	}

	// 0. 优先响应已取消的 Context
	if err := ctx.Err(); err != nil {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("请求已取消: %v", err),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	// 1. 全局最多并发 2 个
	select {
	case <-ctx.Done():
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("请求已取消: %v", ctx.Err()),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	default:
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         ErrAIBusy.Error(),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	// 2. 校验设备截图有效性：缺少有效截图不得发送请求或伪造视觉决策
	if strings.TrimSpace(state.Screenshot.DataBase64) == "" || state.Screenshot.Width <= 0 || state.Screenshot.Height <= 0 {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         "缺少有效设备截图，无法进行视觉决策",
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	// 2. 读取配置
	endpoint, _ := c.database.GetConfig(ConfigKeyAIEndpoint)
	model, _ := c.database.GetConfig(ConfigKeyAIModel)
	apiKey, _ := c.database.GetConfig(ConfigKeyAIApiKey)
	timeoutSec := DefaultAITimeoutSeconds
	if tStr, _ := c.database.GetConfig(ConfigKeyAITimeout); tStr != "" {
		var t int
		if _, err := fmt.Sscanf(tStr, "%d", &t); err == nil && t > 0 {
			timeoutSec = t
		}
	}

	endpoint = strings.TrimSpace(endpoint)
	model = strings.TrimSpace(model)
	if endpoint == "" || model == "" {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         ErrAINotConfigured.Error(),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	// 3. 超时上下文控制（默认 30s）
	aiCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 4. 决策目标确定
	goal := strings.TrimSpace(requestedGoal)
	goalNotice := ""
	if goal == "" {
		goalNotice = "【提示：当前未提供显式任务目标；请仅对当前界面元素、可能的操作入口与页面状态进行中立客观分析，不要臆造或猜测用户想要达成的目标。】"
	}

	// 5. 组装 Prompt
	var ocrSummary []string
	for _, b := range state.OCR.Boxes {
		if strings.TrimSpace(b.Text) != "" {
			ocrSummary = append(ocrSummary, b.Text)
		}
	}
	ocrStr := strings.Join(ocrSummary, " | ")
	if len(ocrStr) > 500 {
		ocrStr = ocrStr[:500] + "..."
	}

	var matchSummary []string
	for _, m := range state.TemplateMatches {
		matchSummary = append(matchSummary, fmt.Sprintf("%s(%s, conf:%.2f)", m.TemplateName, m.Source, m.Confidence))
	}
	matchStr := strings.Join(matchSummary, ", ")
	if matchStr == "" {
		matchStr = "无"
	}

	pkgName := "未知"
	actName := "未知"
	if state.PageInfo != nil {
		pkgName = state.PageInfo.PackageName
		actName = state.PageInfo.Activity
	}

	userTextPrompt := fmt.Sprintf(`你是移动端自动化视觉分析助手。
设备序列号: %s
当前前台应用包名: %s
当前 Activity: %s
观测稳定性: %v
目标: %s %s
页面 OCR 文本提取: %s
模板命中: %s

请仔细分析附带的屏幕截图，结合上述应用上下文和目标，以 JSON 格式返回分析结果，必须严格包含以下字段：
{
  "page_summary": "当前画面的核心内容描述",
  "visible_controls_and_dialogs": ["可见的重要按钮、输入框、标签页或弹窗"],
  "task_progress_assessment": "根据目标判断任务当前进展或停顿阶段",
  "next_step_suggestion": "建议下一步操作（例如：点击某按钮、返回、滑动、等待），仅供参考，不触发设备动作",
  "rationale": "做出该建议的依据",
  "uncertainties": ["对当前界面的疑问、遮挡或不确定项"]
}`, state.Serial, pkgName, actName, state.ContextStable, goal, goalNotice, ocrStr, matchStr)

	// 6. 构造 OpenAI Vision 请求体
	reqBody := map[string]any{
		"model": model,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": userTextPrompt,
					},
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": state.Screenshot.DataBase64,
						},
					},
				},
			},
		},
		"response_format": map[string]any{
			"type": "json_object",
		},
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("序列化请求失败: %v", err),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	httpReq, err := http.NewRequestWithContext(aiCtx, http.MethodPost, endpoint, bytes.NewReader(reqData))
	if err != nil {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("创建 HTTP 请求失败: %v", err),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		errStr := err.Error()
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errStr, "deadline exceeded") {
			errStr = fmt.Sprintf("AI 请求超时 (超过预算 %d 秒)", timeoutSec)
		} else if errors.Is(err, context.Canceled) {
			errStr = "AI 请求已取消"
		}
		errStr = sanitizeSecret(errStr, apiKey)
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         errStr,
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("读取 AI 响应失败: %v", err),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respData, &errResp)
		msg := errResp.Error.Message
		if msg == "" {
			msg = string(respData)
		}
		msg = sanitizeSecret(msg, apiKey)
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("AI 服务返回状态码 %d: %s", resp.StatusCode, msg),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	// 解析 OpenAI Completions 响应
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &completion); err != nil || len(completion.Choices) == 0 {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("AI 响应格式解析失败: %v", err),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	var structured struct {
		PageSummary     string   `json:"page_summary"`
		VisibleControls []string `json:"visible_controls_and_dialogs"`
		TaskProgress    string   `json:"task_progress_assessment"`
		NextStep        string   `json:"next_step_suggestion"`
		Rationale       string   `json:"rationale"`
		Uncertainties   []string `json:"uncertainties"`
	}
	if err := json.Unmarshal([]byte(content), &structured); err != nil {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         fmt.Sprintf("AI 输出非合法结构化 JSON: %v", err),
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	if strings.TrimSpace(structured.PageSummary) == "" &&
		strings.TrimSpace(structured.NextStep) == "" &&
		strings.TrimSpace(structured.TaskProgress) == "" &&
		len(structured.VisibleControls) == 0 {
		return AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         "模型返回空决策内容",
			DurationMS:    time.Since(start).Milliseconds(),
		}
	}

	return AIDecisionResult{
		Available:       true,
		ObservationID:   state.ObservationID,
		PageSummary:     structured.PageSummary,
		VisibleControls: structured.VisibleControls,
		TaskProgress:    structured.TaskProgress,
		NextStep:        structured.NextStep,
		Rationale:       structured.Rationale,
		Uncertainties:   structured.Uncertainties,
		Model:           model,
		DurationMS:      time.Since(start).Milliseconds(),
	}
}
