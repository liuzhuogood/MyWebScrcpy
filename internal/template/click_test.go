package template

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mywebscrcpy/internal/action"
)

type mockActionSubmitter struct {
	mu       sync.Mutex
	requests []action.Request
}

func (m *mockActionSubmitter) SubmitAction(ctx context.Context, serial string, req action.Request) (action.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, req)
	return action.Result{
		RequestID: req.RequestID,
		Accepted:  true,
		Executed:  true,
	}, nil
}

func (m *mockActionSubmitter) LastRequest() *action.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return nil
	}
	return &m.requests[len(m.requests)-1]
}

func createClickTestScene(w, h int, targetX, targetY, targetW, targetH int) (*image.RGBA, *image.RGBA) {
	scene := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			scene.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}

	tmpl := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			col := color.RGBA{R: 20, G: 180, B: 50, A: 255}
			if (x+y)%4 == 0 {
				col = color.RGBA{R: 250, G: 50, B: 50, A: 255}
			}
			tmpl.Set(x, y, col)
			scene.Set(targetX+x, targetY+y, col)
		}
	}

	return scene, tmpl
}

func TestService_ClickTemplate(t *testing.T) {
	dir := t.TempDir()
	storage, err := NewStorage(dir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	mockSubmit := &mockActionSubmitter{}
	svc := NewService("adb", storage, nil)
	svc.SetActionSubmitter(mockSubmit)

	sceneW, sceneH := 200, 400
	tX, tY, tW, tH := 50, 100, 40, 40
	sceneImg, tmplImg := createClickTestScene(sceneW, sceneH, tX, tY, tW, tH)

	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return sceneImg, nil
	})

	// 1. Create template
	tmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		Name:      "login_button",
		Serial:    "dev_click",
		Threshold: 0.85,
	}, tmplImg)
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}

	ctx := context.Background()

	// 2. Click by template name, without random offset (exact center)
	res, err := svc.ClickTemplate(ctx, "dev_click", "login_button", ClickOptions{
		RandomOffset: false,
		Mode:         "adb",
	})
	if err != nil {
		t.Fatalf("ClickTemplate failed: %v", err)
	}

	if !res.OK || !res.Executed {
		t.Fatalf("unexpected click result: %+v", res)
	}
	if res.TemplateID != tmpl.ID || res.TemplateName != "login_button" {
		t.Fatalf("template metadata mismatch: %+v", res)
	}

	// Expected center: (50 + 20)/200 = 0.35, (100 + 20)/400 = 0.30
	expectedX := float64(tX+tW/2) / float64(sceneW)
	expectedY := float64(tY+tH/2) / float64(sceneH)
	if diff := res.X - expectedX; diff < -0.01 || diff > 0.01 {
		t.Fatalf("expected X ~ %f, got %f", expectedX, res.X)
	}
	if diff := res.Y - expectedY; diff < -0.01 || diff > 0.01 {
		t.Fatalf("expected Y ~ %f, got %f", expectedY, res.Y)
	}

	// Verify action submitted
	lastReq := mockSubmit.LastRequest()
	if lastReq == nil || lastReq.Action != "tap" || lastReq.Mode != "adb" {
		t.Fatalf("action submitter received invalid request: %+v", lastReq)
	}

	// 3. Click with random offset (must stay within target bounding box)
	resOffset, err := svc.ClickTemplate(ctx, "dev_click", tmpl.ID, ClickOptions{
		RandomOffset: true,
		Mode:         "sdk",
		Humanize:     true,
	})
	if err != nil {
		t.Fatalf("ClickTemplate with offset failed: %v", err)
	}

	minX := float64(tX) / float64(sceneW)
	maxX := float64(tX+tW) / float64(sceneW)
	minY := float64(tY) / float64(sceneH)
	maxY := float64(tY+tH) / float64(sceneH)

	if resOffset.X < minX || resOffset.X > maxX {
		t.Fatalf("offset X %f out of box [%f, %f]", resOffset.X, minX, maxX)
	}
	if resOffset.Y < minY || resOffset.Y > maxY {
		t.Fatalf("offset Y %f out of box [%f, %f]", resOffset.Y, minY, maxY)
	}

	// 4. Click non-existent template
	_, err = svc.ClickTemplate(ctx, "dev_click", "non_existent_btn", ClickOptions{})
	if err == nil || err != ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}

	// 5. Template not matched on screen
	blankScene := image.NewRGBA(image.Rect(0, 0, sceneW, sceneH))
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return blankScene, nil
	})
	_, err = svc.ClickTemplate(ctx, "dev_click", "login_button", ClickOptions{})
	if err == nil || err != ErrTemplateNotMatched {
		t.Fatalf("expected ErrTemplateNotMatched, got %v", err)
	}
}

func TestHandler_ClickTemplateAPI(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewStorage(dir)
	mockSubmit := &mockActionSubmitter{}
	svc := NewService("adb", storage, nil)
	svc.SetActionSubmitter(mockSubmit)

	sceneW, sceneH := 200, 400
	tX, tY, tW, tH := 50, 100, 40, 40
	sceneImg, tmplImg := createClickTestScene(sceneW, sceneH, tX, tY, tW, tH)

	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return sceneImg, nil
	})

	tmpl, _ := storage.CreateTemplate(CreateTemplateRequest{
		Name:      "confirm_dialog_btn",
		Serial:    "phone_1",
		Threshold: 0.85,
	}, tmplImg)

	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)

	// 1. Success click via JSON
	reqBody := ClickRequest{
		Serial:       "phone_1",
		Name:         "confirm_dialog_btn",
		RandomOffset: true,
		Mode:         "adb",
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/templates/click", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var res ClickResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal click result: %v", err)
	}
	if !res.OK || res.TemplateID != tmpl.ID || res.Mode != "adb" {
		t.Fatalf("unexpected click result: %+v", res)
	}

	// 2. Success click via query/form
	reqForm := httptest.NewRequest(http.MethodPost, "/api/templates/click?serial=phone_1&name=confirm_dialog_btn", nil)
	wForm := httptest.NewRecorder()
	mux.ServeHTTP(wForm, reqForm)
	if wForm.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for query params, got %d: %s", wForm.Code, wForm.Body.String())
	}

	// 3. Not found template (404)
	reqNotFound := httptest.NewRequest(http.MethodPost, "/api/templates/click?serial=phone_1&name=unknown_button", nil)
	wNotFound := httptest.NewRecorder()
	mux.ServeHTTP(wNotFound, reqNotFound)
	if wNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d: %s", wNotFound.Code, wNotFound.Body.String())
	}

	// 4. Not matched on screen (422)
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return image.NewRGBA(image.Rect(0, 0, sceneW, sceneH)), nil
	})
	reqNotMatched := httptest.NewRequest(http.MethodPost, "/api/templates/click?serial=phone_1&name=confirm_dialog_btn", nil)
	wNotMatched := httptest.NewRecorder()
	mux.ServeHTTP(wNotMatched, reqNotMatched)
	if wNotMatched.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity, got %d: %s", wNotMatched.Code, wNotMatched.Body.String())
	}
	if !strings.Contains(wNotMatched.Body.String(), "template_not_matched") {
		t.Fatalf("expected template_not_matched error code, got %s", wNotMatched.Body.String())
	}
}
