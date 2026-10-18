package devicestate

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/template"
	"mywebscrcpy/internal/uipage"
	"mywebscrcpy/internal/visionframes"
)

func createTestPNG(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func createTestJPEG(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func setupTestDeviceStateEnvironment(t *testing.T) (*Service, *AIClient, *db.DB, *template.Storage, string) {
	t.Helper()
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	tmplDir := filepath.Join(tempDir, "templates")
	_ = os.MkdirAll(tmplDir, 0755)
	tmplStorage, err := template.NewStorage(tmplDir)
	if err != nil {
		t.Fatal(err)
	}

	aiClient := NewAIClient(database)

	// 使用注入 Runner 的 uiPage 服务
	uiPageSvc := uipage.NewWithRunner(func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("mCurrentFocus=Window{123 u0 com.example.app/com.example.app.MainActivity}"), nil
	})

	svc := NewService(
		"adb",
		uiPageSvc,
		nil,
		tmplStorage,
		aiClient,
		visionframes.NewStore(),
	)

	return svc, aiClient, database, tmplStorage, tempDir
}

func TestAcceptHeaderContentNegotiation(t *testing.T) {
	svc, aiClient, _, _, _ := setupTestDeviceStateEnvironment(t)
	handler := NewHandler(svc, aiClient)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. 默认 Accept 为 JSON
	reqJSON := httptest.NewRequest("GET", "/api/device-state/dev-test-1", nil)
	wJSON := httptest.NewRecorder()
	mux.ServeHTTP(wJSON, reqJSON)
	if wJSON.Code != http.StatusOK {
		t.Fatalf("状态码错误: %d", wJSON.Code)
	}
	if !strings.Contains(wJSON.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("期望 JSON，实际: %s", wJSON.Header().Get("Content-Type"))
	}
	var state DeviceState
	if err := json.Unmarshal(wJSON.Body.Bytes(), &state); err != nil {
		t.Fatalf("反序列化 JSON 失败: %v", err)
	}
	if state.ObservationID == "" {
		t.Fatal("缺少 observation_id")
	}

	// 2. Accept: text/html 返回简单服务端 HTML
	reqHTML := httptest.NewRequest("GET", "/api/device-state/dev-test-1", nil)
	reqHTML.Header.Set("Accept", "text/html,application/xhtml+xml")
	wHTML := httptest.NewRecorder()
	mux.ServeHTTP(wHTML, reqHTML)
	if wHTML.Code != http.StatusOK {
		t.Fatalf("状态码错误: %d", wHTML.Code)
	}
	if !strings.Contains(wHTML.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("期望 HTML，实际: %s", wHTML.Header().Get("Content-Type"))
	}
	htmlBody := wHTML.Body.String()
	if !strings.Contains(htmlBody, "<title>设备总状态 · dev-test-1</title>") {
		t.Fatalf("HTML 内容缺少标题: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "手动刷新") {
		t.Fatalf("HTML 内容缺少手动刷新入口")
	}
}

func TestThreeSourceTemplatesIsolation(t *testing.T) {
	svc, _, _, tmplStorage, tempDir := setupTestDeviceStateEnvironment(t)

	// 创建全局模板: "same-name"
	pngBytes := createTestPNG(10, 10, color.RGBA{R: 255, A: 255})
	tmplImg, _ := png.Decode(bytes.NewReader(pngBytes))
	_, _ = tmplStorage.CreateTemplate(template.CreateTemplateRequest{
		ID:        "tmpl-global",
		Name:      "same-name",
		Serial:    "global",
		Threshold: 0.85,
	}, tmplImg)

	// 创建设备模板: "same-name"
	_, _ = tmplStorage.CreateTemplate(template.CreateTemplateRequest{
		ID:        "tmpl-device",
		Name:      "same-name",
		Serial:    "phone-a",
		Threshold: 0.90,
	}, tmplImg)

	// 创建技能模板: "same-name"
	skillDir := filepath.Join(tempDir, "my-skill")
	skillTmplDir := filepath.Join(skillDir, "templates", "tmpl-skill")
	_ = os.MkdirAll(skillTmplDir, 0755)
	_ = os.WriteFile(filepath.Join(skillTmplDir, "meta.json"), []byte(`{"name":"same-name","threshold":0.95}`), 0644)
	_ = os.WriteFile(filepath.Join(skillTmplDir, "template.png"), pngBytes, 0644)

	// 采集总状态
	state, err := svc.CollectState(t.Context(), "phone-a", nil)
	if err != nil {
		t.Fatalf("CollectState 失败: %v", err)
	}
	if state == nil || state.Serial != "phone-a" {
		t.Fatalf("采集状态无效: %+v", state)
	}

	// 验证模板合并列表：全局、设备、技能三类模板均加载，同名分别保留
	allTemplates, _ := svc.collectAllTemplates("phone-a", skillDir)
	if len(allTemplates) != 3 {
		t.Fatalf("期望加载 3 个模板，实际 %d", len(allTemplates))
	}

	sourceCount := map[string]int{}
	for _, tItem := range allTemplates {
		sourceCount[tItem.Source]++
		if tItem.Name != "same-name" {
			t.Fatalf("模板名称应保留为 same-name: %s", tItem.Name)
		}
	}
	if sourceCount["global"] != 1 || sourceCount["device"] != 1 || sourceCount["skill"] != 1 {
		t.Fatalf("三类模板来源隔离计数错误: %+v", sourceCount)
	}
}

func TestAIDecisionHTTPFixture(t *testing.T) {
	// 创建 Mock AI HTTP 服务
	var lastPromptText string
	var lastImageURL string
	var receivedAuth string
	var mu sync.Mutex

	mockAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedAuth = r.Header.Get("Authorization")
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content []struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Messages) > 0 {
			for _, part := range body.Messages[0].Content {
				if part.Type == "text" {
					lastPromptText = part.Text
				} else if part.Type == "image_url" {
					lastImageURL = part.ImageURL.URL
				}
			}
		}
		mu.Unlock()

		respJSON := `{
			"id": "chatcmpl-test",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"page_summary\":\"首页已打开\",\"visible_controls_and_dialogs\":[\"登录按钮\",\"立即开始\"],\"task_progress_assessment\":\"任务处于初始页\",\"next_step_suggestion\":\"点击登录按钮\",\"rationale\":\"需先认证\",\"uncertainties\":[\"是否有验证码\"]}"
				}
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respJSON))
	}))
	defer mockAIServer.Close()

	_, aiClient, dbInstance, _, _ := setupTestDeviceStateEnvironment(t)

	// 配置 AI 端点与模型
	_ = dbInstance.SetConfig(ConfigKeyAIEndpoint, mockAIServer.URL)
	_ = dbInstance.SetConfig(ConfigKeyAIModel, "gpt-4o-mini-fixture")
	_ = dbInstance.SetConfig(ConfigKeyAIApiKey, "sk-test-secret-key-12345")

	state := &DeviceState{
		ObservationID: "obs-fixture-1",
		Serial:        "device-mock-1",
		Screenshot: ScreenshotInfo{
			DataBase64: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			Width:      1,
			Height:     1,
		},
	}

	// 1. 发起 AI 决策请求（带显式 goal）
	decision := aiClient.Decide(t.Context(), state, "测试登录流程")
	if !decision.Available {
		t.Fatalf("AI 决策应可用，但失败: %s", decision.Error)
	}
	if decision.ObservationID != "obs-fixture-1" {
		t.Fatalf("决策结果未绑定当前观测标识: %s", decision.ObservationID)
	}
	if decision.NextStep != "点击登录按钮" || decision.PageSummary != "首页已打开" {
		t.Fatalf("决策结构化解析错误: %+v", decision)
	}
	if len(decision.VisibleControls) != 2 || len(decision.Uncertainties) != 1 {
		t.Fatalf("控件与不确定项解析异常: %+v", decision)
	}

	mu.Lock()
	auth := receivedAuth
	imgURL := lastImageURL
	prompt := lastPromptText
	mu.Unlock()

	if auth != "Bearer sk-test-secret-key-12345" {
		t.Fatalf("未携带合法的 Authorization 头部: %s", auth)
	}
	if !strings.HasPrefix(imgURL, "data:image/png;base64,") {
		t.Fatalf("图片协议非 Base64 data URL: %s", imgURL)
	}
	if !strings.Contains(prompt, "测试登录流程") {
		t.Fatalf("Prompt 缺少指定的 goal")
	}

	// 2. 验证缺失 goal 时的中立分析提示
	decisionNoGoal := aiClient.Decide(t.Context(), state, "")
	if !decisionNoGoal.Available {
		t.Fatalf("无 goal 时决策应正常返回: %v", decisionNoGoal.Error)
	}
	mu.Lock()
	promptNoGoal := lastPromptText
	mu.Unlock()
	if !strings.Contains(promptNoGoal, "当前未提供显式任务目标") {
		t.Fatalf("缺少 goal 时未注入中立客观提示: %s", promptNoGoal)
	}
}

func TestAIConcurrencyLimitAndTimeout(t *testing.T) {
	// 慢速 Mock 服务用于测试并发与超时
	blockCh := make(chan struct{})
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blockCh
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer slowServer.Close()

	svc, aiClient, dbInstance, _, _ := setupTestDeviceStateEnvironment(t)
	_ = dbInstance.SetConfig(ConfigKeyAIEndpoint, slowServer.URL)
	_ = dbInstance.SetConfig(ConfigKeyAIModel, "mock-model")
	_ = dbInstance.SetConfig(ConfigKeyAITimeout, "1") // 1 秒超时

	state := &DeviceState{ObservationID: "obs-concurrency"}

	// 占满 2 个并发槽位
	aiClient.semaphore <- struct{}{}
	aiClient.semaphore <- struct{}{}

	// 第 3 个请求应立即被拒绝（超出限制 2）
	decBusy := aiClient.Decide(t.Context(), state, "并发测试")
	if decBusy.Available || !strings.Contains(decBusy.Error, "最大并发限制") {
		t.Fatalf("未正确拦截并发超额请求: %+v", decBusy)
	}

	// 释放槽位
	<-aiClient.semaphore
	<-aiClient.semaphore

	// 测试超时取消
	cancelCtx, cancel := context.WithCancel(t.Context())
	cancel() // 立即取消
	decCanceled := aiClient.Decide(cancelCtx, state, "取消测试")
	if decCanceled.Available || !strings.Contains(decCanceled.Error, "取消") {
		t.Fatalf("未正确处理请求取消: %+v", decCanceled)
	}

	close(blockCh)
	_ = svc
}

func TestServiceConfigMasking(t *testing.T) {
	_, aiClient, dbInstance, _, _ := setupTestDeviceStateEnvironment(t)

	// 1. 设置真实密钥
	_ = dbInstance.SetConfig(ConfigKeyAIEndpoint, "https://api.openai.com/v1/chat/completions")
	_ = dbInstance.SetConfig(ConfigKeyAIModel, "gpt-4o-mini")
	_ = dbInstance.SetConfig(ConfigKeyAIApiKey, "sk-proj-1234567890abcdef")

	cfg, err := aiClient.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	// 验证密钥被掩码脱敏
	if cfg.AIApiKeyMasked == "sk-proj-1234567890abcdef" || !strings.Contains(cfg.AIApiKeyMasked, "****") {
		t.Fatalf("API 密钥未被掩码脱敏: %s", cfg.AIApiKeyMasked)
	}
	if !cfg.AIHasKey {
		t.Fatal("AIHasKey 应为 true")
	}

	// 2. 传入带有掩码的配置保存时，真实密钥不被覆盖
	err = aiClient.SaveConfig(ServiceConfig{
		AIChatEndpoint:   "https://api.example.com/v1/chat/completions",
		AIModel:          "claude-3-haiku",
		AIApiKeyMasked:   cfg.AIApiKeyMasked, // 带有 ****
		AITimeoutSeconds: 25,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 真实密钥保持原值
	actualKey, _ := dbInstance.GetConfig(ConfigKeyAIApiKey)
	if actualKey != "sk-proj-1234567890abcdef" {
		t.Fatalf("真实密钥被掩码覆盖: %s", actualKey)
	}

	// 3. 传入新明文密钥时，真实密钥更新
	err = aiClient.SaveConfig(ServiceConfig{
		AIApiKeyMasked: "sk-new-secret-999999",
	})
	if err != nil {
		t.Fatal(err)
	}
	newActualKey, _ := dbInstance.GetConfig(ConfigKeyAIApiKey)
	if newActualKey != "sk-new-secret-999999" {
		t.Fatalf("新密钥未保存成功: %s", newActualKey)
	}
}

func TestDeviceStateEdgeCasesAndProbeChecks(t *testing.T) {
	// 1. 数据库为 nil 时 Decide 不应 panic
	aiNil := NewAIClient(nil)
	dec := aiNil.Decide(context.Background(), &DeviceState{ObservationID: "test"}, "")
	if dec.Available || dec.Error == "" {
		t.Fatalf("数据库为 nil 时应返回不可用: %+v", dec)
	}

	// 2. 空决策 JSON 处理
	temp := t.TempDir()
	database, _ := db.Open(filepath.Join(temp, "test.db"))
	defer database.Close()
	mode := "empty"
	secretKey := "test-secret-never-leak-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if mode == "echo" {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "rejected: " + secretKey},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": "{}"}}},
		})
	}))
	defer server.Close()

	ai := NewAIClient(database)
	_ = ai.SaveConfig(ServiceConfig{
		AIChatEndpoint:   server.URL,
		AIModel:          "fixture-vision",
		AIApiKeyMasked:   secretKey,
		AITimeoutSeconds: 5,
	})

	state := &DeviceState{
		ObservationID: "obs-edge",
		Serial:        "dev-edge",
		Screenshot: ScreenshotInfo{
			DataBase64: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nmS8AAAAASUVORK5CYII=",
			Width:      1,
			Height:     1,
		},
	}

	emptyDec := ai.Decide(context.Background(), state, "测试")
	if emptyDec.Available {
		t.Fatal("空决策 JSON 不应标记为可用")
	}

	// 3. 上游报错脱敏
	mode = "echo"
	echoDec := ai.Decide(context.Background(), state, "")
	if strings.Contains(echoDec.Error, secretKey) {
		t.Fatalf("决策报错中泄漏了配置的 API 密钥: %s", echoDec.Error)
	}
	if !strings.Contains(echoDec.Error, "[REDACTED]") {
		t.Fatalf("报错中应包含 [REDACTED] 脱敏标记: %s", echoDec.Error)
	}

	// 4. HTML 渲染截图不应被转义为 #ZgotmplZ
	var htmlBuf bytes.Buffer
	if err := RenderHTML(&htmlBuf, *state, nil); err != nil {
		t.Fatalf("RenderHTML 失败: %v", err)
	}
	if strings.Contains(htmlBuf.String(), "#ZgotmplZ") {
		t.Fatal("HTML 中截图 Base64 被转义为 #ZgotmplZ")
	}

	// 5. 采集失败时 ContextStable 为 false，DeviceInfo 不为空
	failedSvc := NewService("/nonexistent/test-adb", nil, nil, nil, nil, visionframes.NewStore())
	observed, err := failedSvc.CollectState(context.Background(), "dev-failed", nil)
	if err != nil {
		t.Fatalf("CollectState 返回非预期外层错误: %v", err)
	}
	if observed.ContextStable {
		t.Fatal("ADB 失败时 ContextStable 应为 false")
	}
	if len(observed.DeviceInfo) == 0 || observed.DeviceInfo["serial"] != "dev-failed" {
		t.Fatalf("DeviceInfo 应该包含设备基本信息: %+v", observed.DeviceInfo)
	}

	// 6. 服务配置接口 CSRF 防御
	h := NewHandler(failedSvc, ai)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	csrfReq := httptest.NewRequest("PUT", "/api/system/service-config", strings.NewReader(`{"ai_model":"hack"}`))
	csrfReq.Header.Set("Sec-Fetch-Site", "cross-site")
	csrfW := httptest.NewRecorder()
	mux.ServeHTTP(csrfW, csrfReq)
	if csrfW.Code != http.StatusForbidden {
		t.Fatalf("跨站修改服务配置未被拦截: code=%d", csrfW.Code)
	}
}
