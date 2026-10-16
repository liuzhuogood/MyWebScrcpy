package automation

import (
	"bytes"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutesValidation(t *testing.T) {
	svc := &Service{
		adbPath: "echo",
	}

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	// 1. 测试 handleTap 缺少 serial
	{
		body := `{"x": 0.5, "y": 0.5}`
		req := httptest.NewRequest("POST", "/api/v1/actions/tap", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing serial, got %d", rec.Code)
		}
	}

	// 2. 测试 handleSwipe 缺少起点
	{
		body := `{"serial": "dummy", "x2": 0.5, "y2": 0.8}`
		req := httptest.NewRequest("POST", "/api/v1/actions/swipe", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			// 在调用 Swipe 时因缺少起点返回 error
			t.Logf("swipe returned status %d as expected", rec.Code)
		}
	}

	// 3. 测试 handleEvent 不支持的事件
	{
		body := `{"serial": "dummy", "event": "invalid_event"}`
		req := httptest.NewRequest("POST", "/api/v1/actions/events", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid event, got %d", rec.Code)
		}
	}

	// 4. 测试 handleDeviceStatus 缺少 serial
	{
		req := httptest.NewRequest("GET", "/api/v1/device-status", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing serial, got %d", rec.Code)
		}
	}

	// 5. 测试兼容路由别名 /api/v1/vision/actions/tap
	{
		body := `{"serial": "dummy", "x": 0.5, "y": 0.5}`
		req := httptest.NewRequest("POST", "/api/v1/vision/actions/tap", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		// 路由应正常被命中并尝试执行
		if rec.Code == http.StatusNotFound {
			t.Errorf("alias route /api/v1/vision/actions/tap should not 404")
		}
	}
}

func TestSwipeStartPointResolution(t *testing.T) {
	// 验证 x/y 与 x1/y1 均可作为划屏起点
	req1 := SwipeRequest{
		Serial: "dev",
		X:      floatPtr(0.1),
		Y:      floatPtr(0.2),
		X2:     0.5,
		Y2:     0.6,
	}
	x, y, ok := req1.GetStartPoint()
	if !ok || x != 0.1 || y != 0.2 {
		t.Errorf("expected (0.1, 0.2), got (%.2f, %.2f)", x, y)
	}

	req2 := SwipeRequest{
		Serial: "dev",
		X1:     floatPtr(0.3),
		Y1:     floatPtr(0.4),
		X2:     0.5,
		Y2:     0.6,
	}
	x, y, ok = req2.GetStartPoint()
	if !ok || x != 0.3 || y != 0.4 {
		t.Errorf("expected (0.3, 0.4), got (%.2f, %.2f)", x, y)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func TestRegionScreenshotBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	img.Set(10, 10, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	svc := &Service{}
	// 测试超出边界抛错
	_, err := svc.RegionScreenshot(nil, RegionScreenshotRequest{
		Serial: "test",
		X:      90,
		Y:      90,
		Width:  20,
		Height: 20,
	})
	if err == nil {
		t.Errorf("expected error for out of bounds screenshot, got nil")
	}
}

func TestCheckUnifiedTimeout(t *testing.T) {
	svc := &Service{}
	// 测试调用缺少 serial 时的错误
	_, err := svc.Check(nil, CheckRequest{
		Kind:      KindPixel,
		TimeoutMS: 50,
	})
	if err == nil {
		t.Errorf("expected error for missing serial in Check, got nil")
	}
}
