package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for x := 0; x < 100; x++ {
		for y := 0; y < 50; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestOCRDetectAndRoutes(t *testing.T) {
	mockOCRServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := map[string]any{
			"ocr_result": "开始游戏\n设置",
			"ocr_boxes": []map[string]any{
				{"text": "开始游戏", "confidence": 0.98, "x": 10.0, "y": 10.0, "w": 40.0, "h": 15.0},
				{"text": "设置", "confidence": 0.92, "x": 60.0, "y": 10.0, "w": 30.0, "h": 15.0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer mockOCRServer.Close()

	service := NewService(nil, nil)
	service.httpClient = mockOCRServer.Client()

	pngBytes := createTestPNG()
	ctx := context.Background()

	resp, err := service.uploadToOCR(ctx, pngBytes, mockOCRServer.URL, "")
	if err != nil {
		t.Fatalf("uploadToOCR 失败: %v", err)
	}
	if resp["ocr_result"] != "开始游戏\n设置" {
		t.Errorf("返回内容错误: %v", resp)
	}

	mux := http.NewServeMux()
	service.RegisterRoutes(mux)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/ocr/status", nil)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Errorf("status 接口返回状态码错误: %d", statusRec.Code)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "test.png")
	_, _ = part.Write(pngBytes)
	_ = writer.WriteField("serial", "test-device")
	_ = writer.WriteField("engine", "paddleocr")
	_ = writer.Close()

	ocrReq := httptest.NewRequest(http.MethodPost, "/api/vision/ocr", &body)
	ocrReq.Header.Set("Content-Type", writer.FormDataContentType())
	ocrRec := httptest.NewRecorder()

	t.Setenv("PADDLEOCR_URL", mockOCRServer.URL)

	mux.ServeHTTP(ocrRec, ocrReq)
	if ocrRec.Code != http.StatusOK {
		t.Fatalf("ocr 接口返回错误: %d, body: %s", ocrRec.Code, ocrRec.Body.String())
	}

	var detResp DetectResponse
	if err := json.NewDecoder(ocrRec.Body).Decode(&detResp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if len(detResp.Boxes) != 2 {
		t.Errorf("解析文字框数量错误: %d", len(detResp.Boxes))
	}
	if detResp.Boxes[0].Text != "开始游戏" {
		t.Errorf("首个文字框错误: %s", detResp.Boxes[0].Text)
	}
}

func TestProcessLifetime_StartSurvivesHTTPAndStopWaitsExit(t *testing.T) {
	scratch := t.TempDir()
	t.Chdir(scratch)
	serverDir := filepath.Join(scratch, "packages", "paddleocr-server")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	port := getFreePort(t)
	fakeServerCode := `import sys, http.server, socketserver
port = int(sys.argv[sys.argv.index('--port') + 1])
class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(b'{"status":"ok","ready":true}')
    def log_message(self, format, *args):
        pass
socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(('127.0.0.1', port), Handler) as httpd:
    httpd.serve_forever()
`
	if err := os.WriteFile(filepath.Join(serverDir, "server.py"), []byte(fakeServerCode), 0644); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(scratch, "python-fixture")
	shimCode := "#!/bin/sh\nif [ \"$1\" = \"-B\" ] && [ \"$2\" = \"-c\" ]; then exit 0; fi\nexec python3 \"$@\"\n"
	if err := os.WriteFile(shim, []byte(shimCode), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYWEBSCRCPY_PYTHON", shim)

	service := NewService(nil, nil)
	service.SetPaddlePort(port)
	defer service.StopPaddle()

	mux := http.NewServeMux()
	service.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. 真实 HTTP 请求返回后，子进程仍然存活
	resp, err := http.Post(server.URL+"/api/ocr/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start returned HTTP %d", resp.StatusCode)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !service.Status().PaddleRunning {
			t.Fatal("HTTP start returned 200 but child exited when request finished")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 2. 显式调用 Stop 等待退出，不产生僵尸进程或重复进程
	stopResp, err := http.Post(server.URL+"/api/ocr/stop", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	stopResp.Body.Close()
	if stopResp.StatusCode != http.StatusOK {
		t.Fatalf("stop returned HTTP %d", stopResp.StatusCode)
	}

	if service.Status().PaddleRunning {
		t.Fatal("PaddleOCR should not be running after stop")
	}
}

func TestDetectEmptyResultContract(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ocr_result": "",
			"ocr_boxes":  []any{},
		})
	}))
	defer mockServer.Close()

	t.Setenv("PADDLEOCR_URL", mockServer.URL)
	service := NewService(nil, nil)

	res, err := service.Detect(context.Background(), createTestPNG(), DetectRequest{Engine: "paddleocr"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "not_found" || res.Matched || res.Boxes == nil || len(res.Boxes) != 0 {
		t.Fatalf("empty detection must return status=not_found matched=false boxes=[], got status=%s matched=%v boxes=%#v", res.Status, res.Matched, res.Boxes)
	}
}

func TestDetectKeywords(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ocr_result": "登录\n注册",
			"ocr_boxes": []map[string]any{
				{"text": "登录", "confidence": 0.95, "x": 10.0, "y": 10.0, "w": 30.0, "h": 10.0},
				{"text": "注册", "confidence": 0.90, "x": 50.0, "y": 10.0, "w": 30.0, "h": 10.0},
			},
		})
	}))
	defer mockServer.Close()

	t.Setenv("PADDLEOCR_URL", mockServer.URL)
	service := NewService(nil, nil)

	// 匹配存在的关键词
	resHit, err := service.Detect(context.Background(), createTestPNG(), DetectRequest{
		Engine:   "paddleocr",
		Keywords: []string{"登录"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resHit.Status != "matched" || !resHit.Matched || len(resHit.Boxes) != 1 || resHit.Boxes[0].Text != "登录" {
		t.Fatalf("keyword match failed: status=%s matched=%v boxes=%+v", resHit.Status, resHit.Matched, resHit.Boxes)
	}

	// 搜索不存在的关键词：必须返回 not_found
	resMiss, err := service.Detect(context.Background(), createTestPNG(), DetectRequest{
		Engine:   "paddleocr",
		Keywords: []string{"退出"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resMiss.Status != "not_found" || resMiss.Matched || resMiss.Boxes == nil || len(resMiss.Boxes) != 0 {
		t.Fatalf("keyword miss must return status=not_found matched=false boxes=[], got status=%s matched=%v", resMiss.Status, resMiss.Matched)
	}
}

func TestDetectCroppedROIMapping(t *testing.T) {
	// 验证契约: 已裁剪图片 + bbox 是原屏归一化位置
	// 裁剪图片尺寸 40x20，原屏 bbox [0.1, 0.2, 0.4, 0.4]
	// OCR 返回裁剪图内的局部像素坐标 (x=10, y=5, w=20, h=10)
	// 映射回原屏坐标应为:
	// gx = 0.1 + (10/40)*0.4 = 0.2
	// gy = 0.2 + (5/20)*0.4 = 0.3
	// gw = (20/40)*0.4 = 0.2
	// gh = (10/20)*0.4 = 0.2
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	croppedPNG := buf.Bytes()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ocr_result": "目标按钮",
			"ocr_boxes": []map[string]any{
				{"text": "目标按钮", "confidence": 0.99, "x": 10.0, "y": 5.0, "w": 20.0, "h": 10.0},
			},
		})
	}))
	defer mockServer.Close()

	t.Setenv("PADDLEOCR_URL", mockServer.URL)
	service := NewService(nil, nil)

	res, err := service.Detect(context.Background(), croppedPNG, DetectRequest{
		Engine: "paddleocr",
		BBox:   []float64{0.1, 0.2, 0.4, 0.4},
	})
	if err != nil {
		t.Fatal(err)
	}

	if res.Status != "matched" || !res.Matched || len(res.Boxes) != 1 {
		t.Fatalf("expected 1 matched box, got %+v", res)
	}

	box := res.Boxes[0]
	const eps = 1e-6
	if diff := box.X - 0.2; diff < -eps || diff > eps {
		t.Errorf("box.X expected ~0.2, got %f", box.X)
	}
	if diff := box.Y - 0.3; diff < -eps || diff > eps {
		t.Errorf("box.Y expected ~0.3, got %f", box.Y)
	}
	if diff := box.W - 0.2; diff < -eps || diff > eps {
		t.Errorf("box.W expected ~0.2, got %f", box.W)
	}
	if diff := box.H - 0.2; diff < -eps || diff > eps {
		t.Errorf("box.H expected ~0.2, got %f", box.H)
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func TestProbePaddleDependenciesDetailed(t *testing.T) {
	scratch := t.TempDir()

	// 1. 模拟所有依赖就绪的 python
	mockPyAll := filepath.Join(scratch, "mock-py-all")
	scriptAll := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(mockPyAll, []byte(scriptAll), 0755); err != nil {
		t.Fatal(err)
	}
	ok, missing := probePaddleDependencies(context.Background(), mockPyAll)
	if !ok || missing != "" {
		t.Errorf("expected true, got %v with missing=%s", ok, missing)
	}

	// 2. 模拟缺少 paddle 和 paddleocr
	mockPyMissing := filepath.Join(scratch, "mock-py-missing")
	scriptMissing := "#!/bin/sh\necho 'paddle,paddleocr' >&2\nexit 1\n"
	if err := os.WriteFile(mockPyMissing, []byte(scriptMissing), 0755); err != nil {
		t.Fatal(err)
	}
	ok, missing = probePaddleDependencies(context.Background(), mockPyMissing)
	if ok || !strings.Contains(missing, "paddle") {
		t.Errorf("expected false with paddle missing, got ok=%v missing=%s", ok, missing)
	}

	// 3. 模拟超时
	mockPyHang := filepath.Join(scratch, "mock-py-hang")
	scriptHang := "#!/bin/sh\nexec /bin/sleep 10\n"
	if err := os.WriteFile(mockPyHang, []byte(scriptHang), 0755); err != nil {
		t.Fatal(err)
	}
	shortCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ok, missing = probePaddleDependencies(shortCtx, mockPyHang)
	if ok {
		t.Errorf("expected false on timeout, got ok=%v missing=%s", ok, missing)
	}
}

func TestStartPaddle_WaitsForHealthReady(t *testing.T) {
	// 使用独立临时端口，不影响 8031 或 8080
	port := getFreePort(t)

	scratch := t.TempDir()
	t.Chdir(scratch)
	serverDir := filepath.Join(scratch, "packages", "paddleocr-server")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 启动一个 fake sidecar：前 300ms 返回 503 (initializing)，300ms 后返回 200 (ready)
	serverPy := filepath.Join(serverDir, "server.py")
	fakeServerCode := `import sys, time, http.server, socketserver

port = int(sys.argv[sys.argv.index('--port') + 1])
start_time = time.time()

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/health':
            if time.time() - start_time < 0.3:
                self.send_response(503)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(b'{"detail": "Model is initializing"}')
            else:
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(b'{"status": "ok", "ready": true}')
        else:
            self.send_response(404)
            self.end_headers()
    def log_message(self, format, *args):
        pass

socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(('127.0.0.1', port), Handler) as httpd:
    httpd.serve_forever()
`
	if err := os.WriteFile(serverPy, []byte(fakeServerCode), 0644); err != nil {
		t.Fatal(err)
	}

	// shim python：probe 返回 0，执行 server.py 时使用当前 python
	shim := filepath.Join(scratch, "python-shim")
	shimCode := `#!/bin/sh
if [ "$1" = "-B" ] && [ "$2" = "-c" ]; then
    exit 0
fi
exec python3 "$@"
`
	if err := os.WriteFile(shim, []byte(shimCode), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MYWEBSCRCPY_PYTHON", shim)

	service := NewService(nil, nil)
	service.SetPaddlePort(port)
	defer service.StopPaddle()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// StartPaddle 必须等待轮询 /health 成功返回 ready 后才返回 nil
	start := time.Now()
	err := service.StartPaddle(ctx)
	if err != nil {
		t.Fatalf("StartPaddle 失败: %v", err)
	}

	// 确认经历了等待 ready 过程 (至少 300ms)
	elapsed := time.Since(start)
	if elapsed < 250*time.Millisecond {
		t.Errorf("StartPaddle 没有真正等待模型 ready: elapsed=%v", elapsed)
	}

	status := service.Status()
	if !status.PaddleRunning || !status.PaddleReady || status.PaddleStatus != "running" {
		t.Errorf("服务应处于运行中状态: %+v", status)
	}

	// 显式停止
	if err := service.StopPaddle(); err != nil {
		t.Fatalf("StopPaddle 失败: %v", err)
	}
	if service.Status().PaddleRunning {
		t.Errorf("服务停止后不应再显示运行")
	}
}

func TestStartPaddle_RejectsWhenDependenciesMissing(t *testing.T) {
	scratch := t.TempDir()
	t.Chdir(scratch)
	serverDir := filepath.Join(scratch, "packages", "paddleocr-server")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	serverPy := filepath.Join(serverDir, "server.py")
	if err := os.WriteFile(serverPy, []byte("# real fake script"), 0644); err != nil {
		t.Fatal(err)
	}

	// 模拟缺少 paddle 依赖的 python 运行时
	shim := filepath.Join(scratch, "python-missing")
	shimCode := `#!/bin/sh
if [ "$1" = "-B" ] && [ "$2" = "-c" ]; then
    echo "paddle,paddleocr" >&2
    exit 1
fi
exit 1
`
	if err := os.WriteFile(shim, []byte(shimCode), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYWEBSCRCPY_PYTHON", shim)

	service := NewService(nil, nil)
	service.SetPaddlePort(getFreePort(t))

	// 启动前必须先拒绝缺依赖，不启动进程
	err := service.StartPaddle(context.Background())
	if err == nil {
		t.Fatal("缺少依赖时 StartPaddle 必须返回错误，不能假报成功")
	}
	if !strings.Contains(err.Error(), "缺少") {
		t.Errorf("错误信息应包含缺少依赖说明: %v", err)
	}

	status := service.Status()
	if status.PaddleRunning {
		t.Errorf("缺少依赖时不能假报运行中: %+v", status)
	}
	if status.PaddleInstalled {
		t.Errorf("缺少依赖时 PaddleInstalled 必须为 false")
	}
}

func TestStartPaddle_ProcessExitsBeforeReady(t *testing.T) {
	scratch := t.TempDir()
	t.Chdir(scratch)
	serverDir := filepath.Join(scratch, "packages", "paddleocr-server")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	serverPy := filepath.Join(serverDir, "server.py")
	if err := os.WriteFile(serverPy, []byte("import sys; sys.exit(1)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	shim := filepath.Join(scratch, "python-exit")
	shimCode := `#!/bin/sh
if [ "$1" = "-B" ] && [ "$2" = "-c" ]; then
    exit 0
fi
exit 1
`
	if err := os.WriteFile(shim, []byte(shimCode), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYWEBSCRCPY_PYTHON", shim)

	service := NewService(nil, nil)
	service.SetPaddlePort(getFreePort(t))

	err := service.StartPaddle(context.Background())
	if err == nil {
		t.Fatal("进程退出前未 ready，必须返回错误")
	}

	status := service.Status()
	if status.PaddleRunning {
		t.Errorf("进程异常退出不能报运行中")
	}
	if status.PaddleStatus != "failed" {
		t.Errorf("进程在 readiness 前退出应标记失败: %+v", status)
	}
}

func TestStartPaddle_TimeoutKillsProcess(t *testing.T) {
	port := getFreePort(t)

	scratch := t.TempDir()
	t.Chdir(scratch)
	serverDir := filepath.Join(scratch, "packages", "paddleocr-server")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 始终返回 503 的 fake server
	serverPy := filepath.Join(serverDir, "server.py")
	fakeServerCode := `import sys, http.server, socketserver

port = int(sys.argv[sys.argv.index('--port') + 1])

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(503)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(b'{"detail": "Never ready"}')
    def log_message(self, format, *args):
        pass

socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(('127.0.0.1', port), Handler) as httpd:
    httpd.serve_forever()
`
	if err := os.WriteFile(serverPy, []byte(fakeServerCode), 0644); err != nil {
		t.Fatal(err)
	}

	shim := filepath.Join(scratch, "python-timeout-shim")
	shimCode := `#!/bin/sh
if [ "$1" = "-B" ] && [ "$2" = "-c" ]; then
    exit 0
fi
exec python3 "$@"
`
	if err := os.WriteFile(shim, []byte(shimCode), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYWEBSCRCPY_PYTHON", shim)

	service := NewService(nil, nil)
	service.SetPaddlePort(port)
	defer service.StopPaddle()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := service.StartPaddle(ctx)
	if err == nil {
		t.Fatal("超时必须返回错误")
	}

	// 超时后必须杀死子进程并等待退出
	time.Sleep(100 * time.Millisecond)
	if service.Status().PaddleRunning {
		t.Errorf("超时失败后子进程应被杀死并清理")
	}
	if service.Status().PaddleStatus != "failed" {
		t.Errorf("超时应保留失败状态: %+v", service.Status())
	}
}
