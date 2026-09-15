package template

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mywebscrcpy/internal/vision"
)

func createTestPNG(path string, width, height int, col color.Color) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, col)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func TestDirWatcher_Direct(t *testing.T) {
	tempDir := t.TempDir()

	notifyCh := make(chan struct{}, 10)
	watcher, err := NewDirWatcher(tempDir, 50*time.Millisecond, func() {
		notifyCh <- struct{}{}
	})
	if err != nil {
		t.Fatalf("NewDirWatcher failed: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("watcher.Start() failed: %v", err)
	}
	defer watcher.Close()

	// 1. Create sub directory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// 2. Create file in sub directory
	testFile := filepath.Join(subDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	select {
	case <-notifyCh:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DirWatcher notification")
	}
}

func TestWatcher_AutoReloadOnDiskChanges(t *testing.T) {
	tempDir := t.TempDir()

	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer storage.Close()

	changeCh := make(chan []string, 20)
	storage.OnChange(func(serials []string) {
		changeCh <- serials
	})

	// 1. Directly write template folder to disk
	tmplDir := filepath.Join(tempDir, "device_test", "disk_tmpl_1")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	tmpl := Template{
		ID:        "disk_tmpl_1",
		Name:      "Disk Template 1",
		Serial:    "device_test",
		Threshold: 0.8,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
		Enabled:   true,
		Width:     20,
		Height:    20,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	metaBytes, _ := json.MarshalIndent(tmpl, "", "  ")
	if err := os.WriteFile(filepath.Join(tmplDir, "meta.json"), metaBytes, 0644); err != nil {
		t.Fatalf("WriteFile meta.json failed: %v", err)
	}

	if err := createTestPNG(filepath.Join(tmplDir, "template.png"), 20, 20, color.RGBA{R: 255, A: 255}); err != nil {
		t.Fatalf("createTestPNG failed: %v", err)
	}

	// Wait for watcher to trigger Reload
	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if tObj, getErr := storage.GetTemplate("disk_tmpl_1"); getErr == nil && tObj != nil {
			found = true
			if tObj.Name != "Disk Template 1" {
				t.Fatalf("expected template name 'Disk Template 1', got %s", tObj.Name)
			}
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		t.Fatal("storage did not auto-reload new template from disk")
	}

	// 2. Modify meta.json on disk
	tmpl.Name = "Modified Template Name"
	tmpl.UpdatedAt = time.Now().UTC()
	metaBytes, _ = json.MarshalIndent(tmpl, "", "  ")
	if err := os.WriteFile(filepath.Join(tmplDir, "meta.json"), metaBytes, 0644); err != nil {
		t.Fatalf("update meta.json failed: %v", err)
	}

	foundUpdated := false
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if tObj, getErr := storage.GetTemplate("disk_tmpl_1"); getErr == nil && tObj.Name == "Modified Template Name" {
			foundUpdated = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !foundUpdated {
		t.Fatal("storage did not auto-reload modified template from disk")
	}

	// 3. Delete template on disk
	if err := os.RemoveAll(tmplDir); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	foundDeleted := false
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, getErr := storage.GetTemplate("disk_tmpl_1"); getErr != nil {
			foundDeleted = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !foundDeleted {
		t.Fatal("storage did not auto-remove deleted template from cache")
	}
}

type testRedetectPublisher struct {
	mu      sync.Mutex
	results map[string][]vision.Message
}

func (p *testRedetectPublisher) PublishVisionResult(serial string, msg vision.Message) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.results[serial] = append(p.results[serial], msg)
}

func TestService_TriggerRedetect(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer storage.Close()

	pub := &testRedetectPublisher{results: make(map[string][]vision.Message)}
	service := NewService("adb", storage, pub)
	defer service.Stop()

	serial := "device_redetect_1"
	_ = service.SetDeviceMatching(serial, true)

	// Create distinctive template and scene containing it
	tmplImg := createDistinctiveTemplate(20, 20)
	scene := createPatternImage(100, 100)
	pasteImage(scene, tmplImg, 40, 40)

	// Feed frame
	_, err = service.ProcessLiveFrame(serial, scene)
	if err != nil {
		t.Fatalf("ProcessLiveFrame failed: %v", err)
	}

	_, err = storage.CreateTemplate(CreateTemplateRequest{
		Name:      "red_box",
		Serial:    serial,
		Threshold: 0.8,
	}, tmplImg)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	// TriggerRedetect should immediately detect against cached frame
	service.TriggerRedetect(serial)

	// Wait for match result
	deadline := time.Now().Add(2 * time.Second)
	matched := false
	for time.Now().Before(deadline) {
		matches := service.GetLatestMatches(serial)
		if len(matches) > 0 {
			matched = true
			if matches[0].Name != "red_box" {
				t.Fatalf("unexpected match label: %s", matches[0].Name)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !matched {
		t.Fatal("TriggerRedetect did not update matches from cached frame")
	}

	// Test TriggerRedetectAll
	service.TriggerRedetectAll()
}
