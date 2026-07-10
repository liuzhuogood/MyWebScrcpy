// cdpshot: 通过 CDP 协议对 9222 浏览器执行 JS 求值和截图的小工具
// 用法:
//   cdpshot eval "<JS表达式>"
//   cdpshot shot <输出png路径> <可选:URL匹配关键字>
//   cdpshot nav <URL>
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

type tab struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	Title                string `json:"title"`
}

func getTabs() ([]tab, error) {
	resp, err := http.Get("http://localhost:9222/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tabs []tab
	json.NewDecoder(resp.Body).Decode(&tabs)
	return tabs, nil
}

func findTab(keyword string) (*tab, error) {
	tabs, err := getTabs()
	if err != nil {
		return nil, err
	}
	for i := range tabs {
		t := &tabs[i]
		if t.Type == "page" && (keyword == "" || strings.Contains(t.URL, keyword)) {
			return t, nil
		}
	}
	return nil, fmt.Errorf("未找到匹配 %q 的标签页", keyword)
}

func newTab(url string) (*tab, error) {
	req, _ := http.NewRequest("PUT", "http://localhost:9222/json/new?"+url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var t tab
	json.NewDecoder(resp.Body).Decode(&t)
	return &t, nil
}

type cdpMsg struct {
	ID     int             `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

func cdpCall(ws *websocket.Conn, ctx context.Context, id int, method string, params interface{}) (json.RawMessage, error) {
	var paramsRaw json.RawMessage
	if params != nil {
		b, _ := json.Marshal(params)
		paramsRaw = b
	}
	msg := cdpMsg{ID: id, Method: method, Params: paramsRaw}
	msgBytes, _ := json.Marshal(msg)
	ws.Write(ctx, websocket.MessageText, msgBytes)
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			return nil, err
		}
		var resp cdpMsg
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		if resp.ID == id {
			return resp.Result, nil
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: cdpshot eval <JS> | shot <png> [关键字] | nav <URL>")
		os.Exit(1)
	}
	cmd := os.Args[1]

	switch cmd {
	case "nav":
		if len(os.Args) < 3 {
			fmt.Println("nav 需要 URL"); os.Exit(1)
		}
		url := os.Args[2]
		t, err := newTab(url)
		if err != nil {
			fmt.Println("打开标签失败:", err); os.Exit(1)
		}
		fmt.Printf("已打开标签: %s\n", t.URL)
		time.Sleep(2 * time.Second)

	case "eval":
		if len(os.Args) < 3 {
			fmt.Println("eval 需要 JS"); os.Exit(1)
		}
		js := os.Args[2]
		keyword := ""
		if len(os.Args) >= 4 {
			keyword = os.Args[3]
		}
		t, err := findTab(keyword)
		if err != nil {
			fmt.Println(err); os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ws, _, err := websocket.Dial(ctx, t.WebSocketDebuggerURL, nil)
		if err != nil {
			fmt.Println("WS 连接失败:", err); os.Exit(1)
		}
		defer ws.Close(websocket.StatusNormalClosure, "")
		result, err := cdpCall(ws, ctx, 1, "Runtime.evaluate", map[string]interface{}{
			"expression":   js,
			"returnByValue": true,
		})
		if err != nil {
			fmt.Println("调用失败:", err); os.Exit(1)
		}
		fmt.Println(string(result))

	case "shot":
		if len(os.Args) < 3 {
			fmt.Println("shot 需要 输出路径"); os.Exit(1)
		}
		out := os.Args[2]
		keyword := ""
		if len(os.Args) >= 4 {
			keyword = os.Args[3]
		}
		t, err := findTab(keyword)
		if err != nil {
			fmt.Println(err); os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ws, _, err := websocket.Dial(ctx, t.WebSocketDebuggerURL, nil)
		if err != nil {
			fmt.Println("WS 连接失败:", err); os.Exit(1)
		}
		defer ws.Close(websocket.StatusNormalClosure, "")
		ws.SetReadLimit(50 << 20) // 50MB，截图很大
		cdpCall(ws, ctx, 1, "Page.enable", nil)
		result, err := cdpCall(ws, ctx, 2, "Page.captureScreenshot", map[string]interface{}{
			"format": "png",
		})
		if err != nil {
			fmt.Println("截图失败:", err); os.Exit(1)
		}
		var shot struct {
			Data string `json:"data"`
		}
		json.Unmarshal(result, &shot)
		img, _ := base64.StdEncoding.DecodeString(shot.Data)
		os.WriteFile(out, img, 0644)
		fmt.Printf("截图已保存: %s (%d bytes)\n", out, len(img))

	default:
		fmt.Println("未知命令:", cmd)
		os.Exit(1)
	}
}

var _ = io.EOF
