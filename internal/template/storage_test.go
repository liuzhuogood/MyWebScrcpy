package template

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func createTestImg(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodeImgToBase64(img image.Image) string {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestStorageCRUD(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmpl_storage_crud_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	testImg := createTestImg(30, 40, color.RGBA{R: 200, G: 100, B: 50, A: 255})

	// 1. Create
	createReq := CreateTemplateRequest{
		ID:        "tmpl-crud-1",
		Name:      "Home Button",
		Serial:    "device-A",
		Threshold: 0.90,
		Method:    MethodTmCcoeffNormed,
	}
	tmpl, err := storage.CreateTemplate(createReq, testImg)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	if tmpl.ID != "tmpl-crud-1" || tmpl.Name != "Home Button" || tmpl.Width != 30 || tmpl.Height != 40 {
		t.Fatalf("Created template attributes mismatch: %+v", tmpl)
	}

	// Verify files on disk
	metaFile := filepath.Join(tempDir, "device-A", "tmpl-crud-1", "meta.json")
	if _, err := os.Stat(metaFile); err != nil {
		t.Errorf("meta.json file missing on disk: %v", err)
	}
	imgFile := filepath.Join(tempDir, "device-A", "tmpl-crud-1", "template.png")
	if _, err := os.Stat(imgFile); err != nil {
		t.Errorf("template.png file missing on disk: %v", err)
	}

	// 2. Get
	fetched, err := storage.GetTemplate("tmpl-crud-1")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if fetched.Name != "Home Button" || fetched.Threshold != 0.90 {
		t.Errorf("Fetched template mismatch: %+v", fetched)
	}

	fetchedImg, err := storage.GetTemplateImage("tmpl-crud-1")
	if err != nil {
		t.Fatalf("GetTemplateImage failed: %v", err)
	}
	if fetchedImg.Bounds().Dx() != 30 || fetchedImg.Bounds().Dy() != 40 {
		t.Errorf("Fetched image dimension mismatch: %v", fetchedImg.Bounds())
	}

	// 3. Update
	newName := "Updated Home Button"
	newThresh := 0.92
	newImg := createTestImg(35, 45, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	updateReq := UpdateTemplateRequest{
		Name:      &newName,
		Threshold: &newThresh,
	}

	updated, err := storage.UpdateTemplate("tmpl-crud-1", updateReq, newImg)
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}
	if updated.Name != newName || updated.Threshold != 0.92 || updated.Width != 35 || updated.Height != 45 {
		t.Errorf("Updated template mismatch: %+v", updated)
	}

	// Verify disk updated
	refetchedImg, err := storage.GetTemplateImage("tmpl-crud-1")
	if err != nil {
		t.Fatalf("GetTemplateImage after update failed: %v", err)
	}
	if refetchedImg.Bounds().Dx() != 35 || refetchedImg.Bounds().Dy() != 45 {
		t.Errorf("Updated image dimensions mismatch: %v", refetchedImg.Bounds())
	}

	// 4. Delete
	if err := storage.DeleteTemplate("tmpl-crud-1"); err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}

	if _, err := storage.GetTemplate("tmpl-crud-1"); err != ErrTemplateNotFound {
		t.Errorf("Expected ErrTemplateNotFound, got %v", err)
	}
	if _, err := storage.GetTemplateImage("tmpl-crud-1"); err != ErrTemplateNotFound {
		t.Errorf("Expected ErrTemplateNotFound for image, got %v", err)
	}

	// Verify disk deleted
	if _, err := os.Stat(filepath.Join(tempDir, "device-A", "tmpl-crud-1")); !os.IsNotExist(err) {
		t.Errorf("Expected directory deleted on disk, but still exists")
	}
}

func TestStorageDeviceIsolation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmpl_storage_iso_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	img := createTestImg(10, 10, color.RGBA{R: 1, G: 2, B: 3, A: 255})

	// 1. Global template
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		ID:     "t-global",
		Name:   "Global Tmpl",
		Serial: "global",
	}, img)
	if err != nil {
		t.Fatalf("Create global template failed: %v", err)
	}

	// 2. Device 1 template
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		ID:     "t-dev1",
		Name:   "Dev1 Tmpl",
		Serial: "device-001",
	}, img)
	if err != nil {
		t.Fatalf("Create dev1 template failed: %v", err)
	}

	// 3. Device 2 template
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		ID:     "t-dev2",
		Name:   "Dev2 Tmpl",
		Serial: "device-002",
	}, img)
	if err != nil {
		t.Fatalf("Create dev2 template failed: %v", err)
	}

	// Query global (serial = "")
	listGlobal := storage.ListTemplates("")
	if len(listGlobal) != 1 || listGlobal[0].ID != "t-global" {
		t.Errorf("Expected only global template for empty serial, got %d", len(listGlobal))
	}

	// Query device-001: should see global + device-001
	listDev1 := storage.ListTemplates("device-001")
	if len(listDev1) != 2 {
		t.Fatalf("Expected 2 templates for device-001 (global + dev1), got %d", len(listDev1))
	}
	ids1 := map[string]bool{listDev1[0].ID: true, listDev1[1].ID: true}
	if !ids1["t-global"] || !ids1["t-dev1"] {
		t.Errorf("Expected t-global and t-dev1, got %+v", listDev1)
	}

	// Query device-002: should see global + device-002
	listDev2 := storage.ListTemplates("device-002")
	if len(listDev2) != 2 {
		t.Fatalf("Expected 2 templates for device-002 (global + dev2), got %d", len(listDev2))
	}
	ids2 := map[string]bool{listDev2[0].ID: true, listDev2[1].ID: true}
	if !ids2["t-global"] || !ids2["t-dev2"] {
		t.Errorf("Expected t-global and t-dev2, got %+v", listDev2)
	}

	// Strict query for device-001
	strictDev1 := storage.ListTemplatesStrict("device-001")
	if len(strictDev1) != 1 || strictDev1[0].ID != "t-dev1" {
		t.Errorf("Expected strictly 1 template for device-001, got %+v", strictDev1)
	}
}

func TestStorageImageValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmpl_storage_val_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// 1. Missing image and missing ImageBase64
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		Name: "No Image",
	}, nil)
	if err == nil {
		t.Errorf("Expected error for missing image, got nil")
	}

	// 2. Corrupt base64
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		Name:        "Corrupt Base64",
		ImageBase64: "not-valid-base64!@",
	}, nil)
	if err == nil {
		t.Errorf("Expected error for invalid base64, got nil")
	}

	// 3. Valid base64 creation
	rawImg := createTestImg(16, 16, color.RGBA{R: 50, G: 60, B: 70, A: 255})
	b64 := encodeImgToBase64(rawImg)
	tmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		ID:          "b64-tmpl",
		Name:        "Base64 Tmpl",
		ImageBase64: b64,
	}, nil)
	if err != nil {
		t.Fatalf("Failed to create template from base64: %v", err)
	}
	if tmpl.Width != 16 || tmpl.Height != 16 {
		t.Errorf("Base64 template dimension mismatch: (%d, %d)", tmpl.Width, tmpl.Height)
	}

	// 4. Invalid method
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		Name:   "Invalid Method",
		Method: "UNKNOWN_METHOD",
	}, rawImg)
	if err == nil {
		t.Errorf("Expected error for unknown method, got nil")
	}

	// 5. Unsafe path traversal in ID or serial
	_, err = storage.CreateTemplate(CreateTemplateRequest{
		ID:     "../escape",
		Name:   "Path Traversal",
		Serial: "global",
	}, rawImg)
	if err == nil {
		t.Errorf("Expected error for path traversal in ID, got nil")
	}
}

func TestStorageConcurrentAccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmpl_storage_concur_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	testImg := createTestImg(12, 12, color.RGBA{R: 88, G: 88, B: 88, A: 255})

	// Pre-create one template
	initTmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		ID:     "concur-base",
		Name:   "Base",
		Serial: "dev-shared",
	}, testImg)
	if err != nil {
		t.Fatalf("Pre-create template failed: %v", err)
	}

	var wg sync.WaitGroup
	workers := 16
	iterations := 25

	for w := 0; w < workers; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Reader
				_, _ = storage.GetTemplate(initTmpl.ID)
				_, _ = storage.GetTemplateImage(initTmpl.ID)
				_ = storage.ListTemplates("dev-shared")

				// Writer: create
				subID := fmt.Sprintf("tmpl-c-%d-%d", workerID, i)
				created, err := storage.CreateTemplate(CreateTemplateRequest{
					ID:     subID,
					Name:   fmt.Sprintf("Worker-%d-%d", workerID, i),
					Serial: "dev-shared",
				}, testImg)

				if err == nil {
					// Writer: update
					newName := fmt.Sprintf("Updated-%d-%d", workerID, i)
					_, _ = storage.UpdateTemplate(created.ID, UpdateTemplateRequest{
						Name: &newName,
					}, nil)

					// Delete
					_ = storage.DeleteTemplate(created.ID)
				}
			}
		}()
	}

	wg.Wait()

	// Base template must still exist and be readable
	base, err := storage.GetTemplate("concur-base")
	if err != nil || base == nil {
		t.Fatalf("Base template lost after concurrent execution: %v", err)
	}
}

func TestStorageReloadFromDisk(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmpl_storage_reload_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testImg := createTestImg(25, 25, color.RGBA{R: 120, G: 200, B: 150, A: 255})

	// Instance 1
	storage1, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage 1 failed: %v", err)
	}

	_, err = storage1.CreateTemplate(CreateTemplateRequest{
		ID:     "reload-global",
		Name:   "Reload Global",
		Serial: "global",
	}, testImg)
	if err != nil {
		t.Fatalf("Create template 1 failed: %v", err)
	}

	_, err = storage1.CreateTemplate(CreateTemplateRequest{
		ID:     "reload-device",
		Name:   "Reload Device",
		Serial: "serial-xyz",
	}, testImg)
	if err != nil {
		t.Fatalf("Create template 2 failed: %v", err)
	}

	// Instance 2 (Reloading existing directory)
	storage2, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage 2 failed: %v", err)
	}

	tGlobal, err := storage2.GetTemplate("reload-global")
	if err != nil {
		t.Fatalf("Failed to retrieve reload-global in storage 2: %v", err)
	}
	if tGlobal.Name != "Reload Global" {
		t.Errorf("Template name mismatch: %s", tGlobal.Name)
	}

	imgGlobal, err := storage2.GetTemplateImage("reload-global")
	if err != nil {
		t.Fatalf("Failed to retrieve reload-global image in storage 2: %v", err)
	}
	if imgGlobal.Bounds().Dx() != 25 || imgGlobal.Bounds().Dy() != 25 {
		t.Errorf("Image dimensions mismatch: %v", imgGlobal.Bounds())
	}

	tDevice, err := storage2.GetTemplate("reload-device")
	if err != nil {
		t.Fatalf("Failed to retrieve reload-device in storage 2: %v", err)
	}
	if tDevice.Serial != "serial-xyz" {
		t.Errorf("Template serial mismatch: %s", tDevice.Serial)
	}
}

func TestStorageNetworkDeviceSerial(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tmpl_storage_net_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	testImg := createTestImg(20, 20, color.RGBA{R: 99, G: 188, B: 210, A: 255})
	networkSerial := "10.0.0.11:5555"

	// 1. Create template with network ADB serial
	tmpl, err := storage.CreateTemplate(CreateTemplateRequest{
		ID:     "net-tmpl-1",
		Name:   "Network ADB Template",
		Serial: networkSerial,
	}, testImg)
	if err != nil {
		t.Fatalf("CreateTemplate with network serial failed: %v", err)
	}

	// 2. Verify tmpl.Serial retains original string with colon
	if tmpl.Serial != networkSerial {
		t.Errorf("Expected Serial to remain %q, got %q", networkSerial, tmpl.Serial)
	}

	// 3. Verify disk directory was sanitized (colon replaced with underscore)
	safeDir := filepath.Join(tempDir, "10.0.0.11_5555", "net-tmpl-1")
	if _, err := os.Stat(safeDir); err != nil {
		t.Errorf("Expected sanitized directory %q on disk, stat error: %v", safeDir, err)
	}
	if _, err := os.Stat(filepath.Join(safeDir, "meta.json")); err != nil {
		t.Errorf("meta.json missing in sanitized directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(safeDir, "template.png")); err != nil {
		t.Errorf("template.png missing in sanitized directory: %v", err)
	}

	// 4. Verify GetTemplate and ListTemplates
	fetched, err := storage.GetTemplate("net-tmpl-1")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if fetched.Serial != networkSerial {
		t.Errorf("Fetched serial mismatch: got %q, want %q", fetched.Serial, networkSerial)
	}

	list := storage.ListTemplates(networkSerial)
	found := false
	for _, item := range list {
		if item.ID == "net-tmpl-1" && item.Serial == networkSerial {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListTemplates(%q) did not return the network template: %+v", networkSerial, list)
	}

	// 5. Verify UpdateTemplate
	newName := "Updated Network ADB Template"
	updated, err := storage.UpdateTemplate("net-tmpl-1", UpdateTemplateRequest{
		Name: &newName,
	}, nil)
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}
	if updated.Name != newName || updated.Serial != networkSerial {
		t.Errorf("Updated template mismatch: %+v", updated)
	}

	// 6. Verify reload from disk restores network serial intact
	storageReloaded, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("Reload NewStorage failed: %v", err)
	}
	reloadedTmpl, err := storageReloaded.GetTemplate("net-tmpl-1")
	if err != nil {
		t.Fatalf("GetTemplate after reload failed: %v", err)
	}
	if reloadedTmpl.Serial != networkSerial {
		t.Errorf("Reloaded serial mismatch: got %q, want %q", reloadedTmpl.Serial, networkSerial)
	}

	// 7. Verify DeleteTemplate removes the sanitized directory
	if err := storageReloaded.DeleteTemplate("net-tmpl-1"); err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}
	if _, err := os.Stat(safeDir); !os.IsNotExist(err) {
		t.Errorf("Expected directory %q removed after deletion", safeDir)
	}
}
