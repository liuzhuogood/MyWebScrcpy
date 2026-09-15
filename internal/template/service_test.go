package template

import (
	"context"
	"errors"
	"image"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"mywebscrcpy/internal/vision"
)

type mockPublisher struct {
	mu       sync.Mutex
	messages []vision.Message
	notifyCh chan vision.Message
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{
		notifyCh: make(chan vision.Message, 50),
	}
}

func (m *mockPublisher) PublishVisionResult(serial string, msg vision.Message) {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	select {
	case m.notifyCh <- msg:
	default:
	}
}

func (m *mockPublisher) getMessages() []vision.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]vision.Message, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func (m *mockPublisher) waitMessage(timeout time.Duration) (vision.Message, error) {
	select {
	case msg := <-m.notifyCh:
		return msg, nil
	case <-time.After(timeout):
		return vision.Message{}, errors.New("timeout waiting for vision message")
	}
}

func setupTestStorage(t *testing.T) (*Storage, string) {
	dir, err := os.MkdirTemp("", "service_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	s, err := NewStorage(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		t.Fatalf("failed to init storage: %v", err)
	}
	return s, dir
}

func TestService_MatchingAndBroadcasting(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	pub := newMockPublisher()
	svc := NewService("mock_adb", storage, pub)
	svc.SetInterval(50 * time.Millisecond)

	// Create template
	tmplImg := createDistinctiveTemplate(20, 20)
	tmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		Name:      "test_button",
		Serial:    "device-1",
		Threshold: 0.85,
	}, tmplImg)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Mock screen capture: returns scene with template pasted at (30, 40)
	scene := createPatternImage(100, 100)
	pasteImage(scene, tmplImg, 30, 40)

	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return scene, nil
	})

	// Enable device matching
	err = svc.SetDeviceMatching("device-1", true)
	if err != nil {
		t.Fatalf("failed to enable matching: %v", err)
	}

	// Wait for published detection result
	msg, err := pub.waitMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("failed to receive detection message: %v", err)
	}

	if msg.Type != "detection.result" {
		t.Errorf("expected type detection.result, got %s", msg.Type)
	}
	if msg.DeviceID != "device-1" {
		t.Errorf("expected device_id device-1, got %s", msg.DeviceID)
	}
	if len(msg.Objects) == 0 {
		t.Fatalf("expected at least 1 detected object, got 0")
	}

	obj := msg.Objects[0]
	if obj.Label != "test_button" {
		t.Errorf("expected label test_button, got %s", obj.Label)
	}
	if obj.Confidence < 0.85 {
		t.Errorf("expected confidence >= 0.85, got %f", obj.Confidence)
	}

	// Check status
	status := svc.GetDeviceStatus("device-1")
	if !status.Enabled {
		t.Errorf("expected status enabled true")
	}
	if status.ActiveTemplates != 1 {
		t.Errorf("expected 1 active template, got %d", status.ActiveTemplates)
	}
	if len(status.LastMatches) == 0 {
		t.Errorf("expected non-empty last matches")
	}

	latest := svc.GetLatestMatches("device-1")
	if len(latest) == 0 {
		t.Errorf("expected non-empty latest matches")
	}
	if latest[0].TemplateID != tmpl.ID {
		t.Errorf("expected template ID %s, got %s", tmpl.ID, latest[0].TemplateID)
	}

	// Cleanup
	svc.Stop()
}

func TestService_NoMatchClearsObjects(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	pub := newMockPublisher()
	svc := NewService("mock_adb", storage, pub)
	svc.SetInterval(50 * time.Millisecond)

	// Create template
	tmplImg := createDistinctiveTemplate(20, 20)
	_, err := storage.CreateTemplate(CreateTemplateRequest{
		Name:      "test_button",
		Serial:    "device-nomatch",
		Threshold: 0.95,
	}, tmplImg)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Scene does NOT contain the template
	scene := createPatternImage(100, 100)
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return scene, nil
	})

	err = svc.SetDeviceMatching("device-nomatch", true)
	if err != nil {
		t.Fatalf("failed to start matching: %v", err)
	}

	msg, err := pub.waitMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("failed to receive clear message: %v", err)
	}

	if len(msg.Objects) != 0 {
		t.Errorf("expected 0 objects on no match, got %d", len(msg.Objects))
	}

	svc.Stop()
}

func TestService_DisableMatchingStopsWorkerAndClears(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	pub := newMockPublisher()
	svc := NewService("mock_adb", storage, pub)
	svc.SetInterval(40 * time.Millisecond)

	tmplImg := createDistinctiveTemplate(20, 20)
	_, _ = storage.CreateTemplate(CreateTemplateRequest{
		Name:   "icon",
		Serial: "dev-toggle",
	}, tmplImg)

	scene := createPatternImage(100, 100)
	pasteImage(scene, tmplImg, 20, 20)
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return scene, nil
	})

	_ = svc.SetDeviceMatching("dev-toggle", true)
	_, err := pub.waitMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("expected message on start: %v", err)
	}

	// Turn off matching
	err = svc.SetDeviceMatching("dev-toggle", false)
	if err != nil {
		t.Fatalf("failed to disable matching: %v", err)
	}

	status := svc.GetDeviceStatus("dev-toggle")
	if status.Enabled {
		t.Errorf("expected status enabled false after disabling")
	}

	// Verify clear message was broadcast
	msgs := pub.getMessages()
	if len(msgs) == 0 {
		t.Fatalf("expected messages published")
	}
	lastMsg := msgs[len(msgs)-1]
	if len(lastMsg.Objects) != 0 {
		t.Errorf("expected empty objects in last message to clear overlays, got %d", len(lastMsg.Objects))
	}

	svc.Stop()
}

func TestService_ConcurrentSwitching(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	pub := newMockPublisher()
	svc := NewService("mock_adb", storage, pub)
	svc.SetInterval(20 * time.Millisecond)

	tmplImg := createDistinctiveTemplate(15, 15)
	_, _ = storage.CreateTemplate(CreateTemplateRequest{
		Name:   "icon",
		Serial: "dev-conc",
	}, tmplImg)

	scene := createPatternImage(80, 80)
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return scene, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			enabled := (idx % 2) == 0
			_ = svc.SetDeviceMatching("dev-conc", enabled)
			_ = svc.GetDeviceStatus("dev-conc")
			_ = svc.GetLatestMatches("dev-conc")
			_, _ = svc.Detect(context.Background(), "dev-conc", scene)
		}(i)
	}
	wg.Wait()

	svc.Stop()
}

func TestService_DetectDirect(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	svc := NewService("adb", storage, nil)

	tmplImg := createDistinctiveTemplate(20, 20)
	tmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		Name:      "direct_btn",
		Serial:    "dev-detect",
		Threshold: 0.85,
	}, tmplImg)
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}

	scene := createPatternImage(100, 100)
	pasteImage(scene, tmplImg, 40, 50)

	// Test Detect
	matches, err := svc.Detect(context.Background(), "dev-detect", scene)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected matches, got 0")
	}
	if matches[0].TemplateID != tmpl.ID {
		t.Errorf("expected template ID %s, got %s", tmpl.ID, matches[0].TemplateID)
	}

	// Test Detect with nil image
	_, err = svc.Detect(context.Background(), "dev-detect", nil)
	if !errors.Is(err, ErrInvalidImage) {
		t.Errorf("expected ErrInvalidImage, got %v", err)
	}

	// Test DetectWithOptions with non-matching template ID
	matchesOpts, err := svc.DetectWithOptions(context.Background(), "dev-detect", scene, []string{"non-existent"}, MatchOptions{})
	if err != nil {
		t.Fatalf("DetectWithOptions failed: %v", err)
	}
	if len(matchesOpts) != 0 {
		t.Errorf("expected 0 matches for non-existent template ID, got %d", len(matchesOpts))
	}
}

func TestService_CaptureScreenDefaultFail(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	svc := NewService("/nonexistent/bin/adb", storage, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := svc.CaptureScreen(ctx, "fake-device")
	if err == nil {
		t.Errorf("expected captureScreen error on invalid adb binary")
	}
}

func TestService_PhysicalResolutionDownscaleAndMatch(t *testing.T) {
	storage, dir := setupTestStorage(t)
	defer os.RemoveAll(dir)

	pub := newMockPublisher()
	svc := NewService("mock_adb", storage, pub)
	svc.SetInterval(30 * time.Millisecond)

	// Template created based on 1024-scale canvas (e.g. 461x1024)
	tmplImg := createDistinctiveTemplate(22, 22)
	tmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		Name:        "login_btn_1024",
		Serial:      "dev-hd",
		Threshold:   0.85,
		SceneWidth:  461,
		SceneHeight: 1024,
	}, tmplImg)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Verify SceneWidth and SceneHeight persisted
	savedTmpl, err := storage.GetTemplate(tmpl.ID)
	if err != nil {
		t.Fatalf("failed to get template: %v", err)
	}
	if savedTmpl.SceneWidth != 461 || savedTmpl.SceneHeight != 1024 {
		t.Errorf("expected scene dimensions 461x1024, got %dx%d", savedTmpl.SceneWidth, savedTmpl.SceneHeight)
	}

	// Simulated physical screen at 1080x2400 (scaled ~2.34x)
	// When downscaled by captureScreenDefault (or scaleImage down to 1024 max dimension),
	// height becomes 1024, width becomes 461.
	scene1024 := createPatternImage(461, 1024)
	pasteImage(scene1024, tmplImg, 100, 200)

	// In the physical screen, everything is 2400/1024 = 2.34375 times larger
	physicalScreen := scaleImage(scene1024, 1080, 2400)

	// Mock screencap that returns the downscaled image (simulating captureScreenDefault downscaling)
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		bounds := physicalScreen.Bounds()
		maxDim := bounds.Dx()
		if bounds.Dy() > maxDim {
			maxDim = bounds.Dy()
		}
		if maxDim > 1024 {
			scale := 1024.0 / float64(maxDim)
			targetW := int(math.Round(float64(bounds.Dx()) * scale))
			targetH := int(math.Round(float64(bounds.Dy()) * scale))
			return scaleImage(physicalScreen, targetW, targetH), nil
		}
		return physicalScreen, nil
	})

	err = svc.SetDeviceMatching("dev-hd", true)
	if err != nil {
		t.Fatalf("failed to enable matching: %v", err)
	}

	msg, err := pub.waitMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("timeout waiting for match result: %v", err)
	}

	if len(msg.Objects) == 0 {
		t.Fatalf("expected matches after 1024 downscaling, got 0")
	}
	if msg.Objects[0].Label != "login_btn_1024" {
		t.Errorf("expected label login_btn_1024, got %s", msg.Objects[0].Label)
	}
	if msg.Objects[0].Confidence < 0.85 {
		t.Errorf("expected confidence >= 0.85, got %f", msg.Objects[0].Confidence)
	}

	svc.Stop()
}

