package template

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// encodeImageToBase64 encodes an image to base64 PNG string.
func encodeImageToBase64(img image.Image) string {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func setupTestServer(t *testing.T) (*httptest.Server, *Service, string) {
	dir, err := os.MkdirTemp("", "handler_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	storage, err := NewStorage(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		t.Fatalf("failed to init storage: %v", err)
	}

	pub := newMockPublisher()
	svc := NewService("mock_adb", storage, pub)
	svc.SetInterval(20 * time.Millisecond)

	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)

	ts := httptest.NewServer(mux)
	return ts, svc, dir
}

func TestHandler_TemplatesCRUD_JSON(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// 1. Initial list should be empty
	resp, err := client.Get(ts.URL + "/api/templates")
	if err != nil {
		t.Fatalf("GET /api/templates failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var listResp struct {
		Templates []*Template `json:"templates"`
		Total     int         `json:"total"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&listResp)
	resp.Body.Close()
	if listResp.Total != 0 {
		t.Errorf("expected 0 templates, got %d", listResp.Total)
	}

	// 2. Create template via JSON
	tmplImg := createDistinctiveTemplate(20, 20)
	b64Img := encodeImageToBase64(tmplImg)

	createReq := CreateTemplateRequest{
		ID:          "tmpl_json_1",
		Name:        "json_btn",
		Serial:      "dev-json",
		Threshold:   0.88,
		ImageBase64: b64Img,
	}
	bodyBytes, _ := json.Marshal(createReq)
	resp, err = client.Post(ts.URL+"/api/templates", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("POST /api/templates failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}
	var created Template
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID != "tmpl_json_1" || created.Name != "json_btn" {
		t.Errorf("unexpected created template: %+v", created)
	}

	// Duplicate ID should fail
	resp, _ = client.Post(ts.URL+"/api/templates", "application/json", bytes.NewReader(bodyBytes))
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate ID, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Get single template
	resp, err = client.Get(ts.URL + "/api/templates/tmpl_json_1")
	if err != nil {
		t.Fatalf("GET /api/templates/tmpl_json_1 failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var fetched Template
	_ = json.NewDecoder(resp.Body).Decode(&fetched)
	resp.Body.Close()
	if fetched.ID != "tmpl_json_1" {
		t.Errorf("expected ID tmpl_json_1, got %s", fetched.ID)
	}

	// Non-existent template
	resp, _ = client.Get(ts.URL + "/api/templates/non_existent")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Download template image
	resp, err = client.Get(ts.URL + "/api/templates/tmpl_json_1/image")
	if err != nil {
		t.Fatalf("GET /api/templates/tmpl_json_1/image failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", ct)
	}
	downloadedImg, err := png.Decode(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to decode downloaded image: %v", err)
	}
	if downloadedImg.Bounds().Dx() != 20 || downloadedImg.Bounds().Dy() != 20 {
		t.Errorf("unexpected image dimensions: %v", downloadedImg.Bounds())
	}

	// 5. Update template via PUT
	newName := "updated_btn"
	newTh := 0.92
	updReq := UpdateTemplateRequest{
		Name:      &newName,
		Threshold: &newTh,
	}
	updBytes, _ := json.Marshal(updReq)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/tmpl_json_1", bytes.NewReader(updBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/templates/tmpl_json_1 failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var updated Template
	_ = json.NewDecoder(resp.Body).Decode(&updated)
	resp.Body.Close()
	if updated.Name != "updated_btn" || updated.Threshold != 0.92 {
		t.Errorf("unexpected updated template: %+v", updated)
	}

	// 6. Delete template
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/templates/tmpl_json_1", nil)
	resp, err = client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /api/templates/tmpl_json_1 failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify it is gone
	resp, _ = client.Get(ts.URL + "/api/templates/tmpl_json_1")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_TemplatesMultipart(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// Prepare multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("name", "multipart_tmpl")
	_ = w.WriteField("serial", "dev-form")
	_ = w.WriteField("threshold", "0.89")
	_ = w.WriteField("scales", "[1.0, 0.8]")
	_ = w.WriteField("enabled", "true")

	part, err := w.CreateFormFile("image", "template.png")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	tmplImg := createDistinctiveTemplate(25, 25)
	_ = png.Encode(part, tmplImg)
	_ = w.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST multipart failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}
	var created Template
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	if created.Name != "multipart_tmpl" || created.Serial != "dev-form" || created.Threshold != 0.89 {
		t.Errorf("unexpected created template: %+v", created)
	}
	if len(created.Scales) != 2 || created.Scales[1] != 0.8 {
		t.Errorf("unexpected scales: %v", created.Scales)
	}

	// Update via multipart
	var bUpd bytes.Buffer
	wUpd := multipart.NewWriter(&bUpd)
	_ = wUpd.WriteField("name", "multipart_renamed")
	_ = wUpd.WriteField("scales", "1.0, 0.5")
	_ = wUpd.Close()

	reqUpd, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/"+created.ID, &bUpd)
	reqUpd.Header.Set("Content-Type", wUpd.FormDataContentType())
	respUpd, err := client.Do(reqUpd)
	if err != nil {
		t.Fatalf("PUT multipart failed: %v", err)
	}
	if respUpd.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", respUpd.StatusCode)
	}
	var updated Template
	_ = json.NewDecoder(respUpd.Body).Decode(&updated)
	respUpd.Body.Close()

	if updated.Name != "multipart_renamed" || len(updated.Scales) != 2 || updated.Scales[1] != 0.5 {
		t.Errorf("unexpected updated template: %+v", updated)
	}
}

func TestHandler_StatusAndMatches(t *testing.T) {
	ts, svc, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// Query status without serial should fail 400
	resp, _ := client.Get(ts.URL + "/api/templates/status")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Query status for dev-status
	resp, err := client.Get(ts.URL + "/api/templates/status?serial=dev-status")
	if err != nil {
		t.Fatalf("GET status failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var status DeviceStatus
	_ = json.NewDecoder(resp.Body).Decode(&status)
	resp.Body.Close()
	if status.Enabled {
		t.Errorf("expected enabled false initially")
	}

	// Turn ON status
	setStatusBody, _ := json.Marshal(map[string]interface{}{
		"serial":  "dev-status",
		"enabled": true,
	})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/status", bytes.NewReader(setStatusBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("PUT status failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = json.NewDecoder(resp.Body).Decode(&status)
	resp.Body.Close()
	if !status.Enabled {
		t.Errorf("expected enabled true after PUT")
	}

	// Query matches
	resp, err = client.Get(ts.URL + "/api/templates/matches?serial=dev-status")
	if err != nil {
		t.Fatalf("GET matches failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var matchResp struct {
		Serial  string        `json:"serial"`
		Matches []MatchResult `json:"matches"`
		Count   int           `json:"count"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&matchResp)
	resp.Body.Close()
	if matchResp.Serial != "dev-status" {
		t.Errorf("expected serial dev-status, got %s", matchResp.Serial)
	}

	// Turn OFF status
	setStatusBodyOff, _ := json.Marshal(map[string]interface{}{
		"serial":  "dev-status",
		"enabled": false,
	})
	reqOff, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/status", bytes.NewReader(setStatusBodyOff))
	reqOff.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(reqOff)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	svc.Stop()
}

func TestHandler_Detect(t *testing.T) {
	ts, svc, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// Register a template
	tmplImg := createDistinctiveTemplate(20, 20)
	tmpl, err := svc.Storage().CreateTemplate(CreateTemplateRequest{
		Name:      "detect_target",
		Serial:    "dev-detect-api",
		Threshold: 0.85,
	}, tmplImg)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Create a scene with the template pasted
	scene := createPatternImage(100, 100)
	pasteImage(scene, tmplImg, 35, 45)
	b64Scene := encodeImageToBase64(scene)

	// 1. Detect via JSON base64
	detectReq := MatchRequest{
		Serial:      "dev-detect-api",
		TemplateIDs: []string{tmpl.ID},
		ImageBase64: b64Scene,
	}
	body, _ := json.Marshal(detectReq)
	resp, err := client.Post(ts.URL+"/api/templates/detect", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/templates/detect failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var detectResp struct {
		Matches []MatchResult `json:"matches"`
		Count   int           `json:"count"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&detectResp)
	resp.Body.Close()

	if detectResp.Count == 0 || len(detectResp.Matches) == 0 {
		t.Fatalf("expected at least 1 match, got 0")
	}
	if detectResp.Matches[0].TemplateID != tmpl.ID {
		t.Errorf("expected matched ID %s, got %s", tmpl.ID, detectResp.Matches[0].TemplateID)
	}

	// Verify handleDetect broadcasts matches
	msgs := svc.publisher.(*mockPublisher).getMessages()
	if len(msgs) == 0 || len(msgs[len(msgs)-1].Objects) == 0 {
		t.Fatalf("expected detection broadcast message sent on detect, got %v", msgs)
	}

	// 2. Detect via multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("serial", "dev-detect-api")
	part, _ := w.CreateFormFile("image", "screen.png")
	_ = png.Encode(part, scene)
	_ = w.Close()

	reqForm, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates/detect", &b)
	reqForm.Header.Set("Content-Type", w.FormDataContentType())
	resp, err = client.Do(reqForm)
	if err != nil {
		t.Fatalf("POST /api/templates/detect form failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	_ = json.NewDecoder(resp.Body).Decode(&detectResp)
	resp.Body.Close()
	if detectResp.Count == 0 {
		t.Errorf("expected matches from multipart detect")
	}

	// 3. Detect via serial capture fallback
	svc.SetCaptureScreenFn(func(ctx context.Context, serial string) (image.Image, error) {
		return scene, nil
	})
	detectReqScreencap := MatchRequest{
		Serial: "dev-detect-api",
	}
	bodySc, _ := json.Marshal(detectReqScreencap)
	resp, err = client.Post(ts.URL+"/api/templates/detect", "application/json", bytes.NewReader(bodySc))
	if err != nil {
		t.Fatalf("POST detect with serial failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	_ = json.NewDecoder(resp.Body).Decode(&detectResp)
	resp.Body.Close()
	if detectResp.Count == 0 {
		t.Errorf("expected matches from screencap detect")
	}

	// 4. Detect missing image and serial
	badReq := MatchRequest{}
	bodyBad, _ := json.Marshal(badReq)
	resp, _ = client.Post(ts.URL+"/api/templates/detect", "application/json", bytes.NewReader(bodyBad))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing image, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_ErrorHandling(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// Invalid JSON on create
	resp, _ := client.Post(ts.URL+"/api/templates", "application/json", bytes.NewReader([]byte("{invalid")))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on invalid json, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Invalid JSON on update
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/dummy", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on invalid json, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Non-existent template image
	resp, _ = client.Get(ts.URL + "/api/templates/dummy/image")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for missing template image, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Missing serial on status PUT
	statusReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/status", bytes.NewReader([]byte(`{"serial":""}`)))
	statusReq.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(statusReq)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on missing serial, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Missing serial on matches GET
	resp, _ = client.Get(ts.URL + "/api/templates/matches")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on missing serial, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_Export(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// 1. Create 3 templates: two with duplicate names on dev-1, one on global
	img1 := createDistinctiveTemplate(20, 20)
	b64_1 := encodeImageToBase64(img1)
	img2 := createDistinctiveTemplate(30, 30)
	b64_2 := encodeImageToBase64(img2)
	img3 := createDistinctiveTemplate(25, 25)
	b64_3 := encodeImageToBase64(img3)

	create := func(id, name, serial, b64 string) {
		req := CreateTemplateRequest{
			ID:          id,
			Name:        name,
			Serial:      serial,
			ImageBase64: b64,
		}
		data, _ := json.Marshal(req)
		resp, err := client.Post(ts.URL+"/api/templates", "application/json", bytes.NewReader(data))
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("failed to create template %s: err=%v, code=%d", id, err, resp.StatusCode)
		}
		resp.Body.Close()
	}

	create("tmpl_1", "home_btn", "dev-1", b64_1)
	create("tmpl_2", "home_btn", "dev-1", b64_2)
	create("tmpl_3", "global_btn", "global", b64_3)

	// 2. Export with scope=device&serial=dev-1
	resp, err := client.Get(ts.URL + "/api/templates/export?scope=device&serial=dev-1")
	if err != nil {
		t.Fatalf("GET export failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %s", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, `attachment; filename="templates-dev-1-`) || !strings.HasSuffix(cd, `.zip"`) {
		t.Errorf("unexpected Content-Disposition: %s", cd)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read zip body: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to parse zip: %v", err)
	}

	if len(zr.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(zr.File))
	}

	expectedNames := map[string]bool{"home_btn.png": true, "home_btn_2.png": true}
	for _, f := range zr.File {
		if !expectedNames[f.Name] {
			t.Errorf("unexpected file in zip: %s", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %s: %v", f.Name, err)
		}
		decoded, err := png.Decode(rc)
		rc.Close()
		if err != nil {
			t.Errorf("failed to decode png from zip entry %s: %v", f.Name, err)
		}
		if f.Name == "home_btn.png" && (decoded.Bounds().Dx() != 20 || decoded.Bounds().Dy() != 20) {
			t.Errorf("unexpected dimensions for home_btn.png: %v", decoded.Bounds())
		}
		if f.Name == "home_btn_2.png" && (decoded.Bounds().Dx() != 30 || decoded.Bounds().Dy() != 30) {
			t.Errorf("unexpected dimensions for home_btn_2.png: %v", decoded.Bounds())
		}
	}

	// 3. Export with scope=global
	respGlobal, err := client.Get(ts.URL + "/api/templates/export?scope=global")
	if err != nil {
		t.Fatalf("GET export global failed: %v", err)
	}
	if respGlobal.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respGlobal.StatusCode)
	}
	zipGlobalBytes, _ := io.ReadAll(respGlobal.Body)
	respGlobal.Body.Close()
	zrGlobal, err := zip.NewReader(bytes.NewReader(zipGlobalBytes), int64(len(zipGlobalBytes)))
	if err != nil {
		t.Fatalf("failed to parse global zip: %v", err)
	}
	if len(zrGlobal.File) != 1 || zrGlobal.File[0].Name != "global_btn.png" {
		t.Errorf("unexpected files in global zip: %+v", zrGlobal.File)
	}

	// 4. Export with scope=all
	respAll, err := client.Get(ts.URL + "/api/templates/export?scope=all")
	if err != nil {
		t.Fatalf("GET export all failed: %v", err)
	}
	if respAll.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respAll.StatusCode)
	}
	zipAllBytes, _ := io.ReadAll(respAll.Body)
	respAll.Body.Close()
	zrAll, err := zip.NewReader(bytes.NewReader(zipAllBytes), int64(len(zipAllBytes)))
	if err != nil {
		t.Fatalf("failed to parse all zip: %v", err)
	}
	if len(zrAll.File) != 3 {
		t.Errorf("expected 3 files in all zip, got %d", len(zrAll.File))
	}
}

func TestHandler_Import(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// 1. Build an in-memory zip archive with multiple items
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	addZipFile := func(name string, content []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		_, _ = w.Write(content)
	}

	// Valid PNG 1
	var p1Buf bytes.Buffer
	_ = png.Encode(&p1Buf, createDistinctiveTemplate(22, 22))
	addZipFile("confirm_ok.png", p1Buf.Bytes())

	// Valid PNG 2 in subfolder
	var p2Buf bytes.Buffer
	_ = png.Encode(&p2Buf, createDistinctiveTemplate(33, 33))
	addZipFile("subfolder/cancel_button.png", p2Buf.Bytes())

	// Ignored entries:
	addZipFile("__MACOSX/._confirm_ok.png", []byte("mac metadata"))
	addZipFile(".hidden.png", p1Buf.Bytes())
	addZipFile("readme.txt", []byte("this is a text file"))
	addZipFile("corrupt.png", []byte("not a png image"))

	_ = zw.Close()

	// 2. Upload zip via POST /api/templates/import
	var formBuf bytes.Buffer
	mw := multipart.NewWriter(&formBuf)
	_ = mw.WriteField("serial", "dev-import")
	part, err := mw.CreateFormFile("file", "templates.zip")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	_, _ = part.Write(zipBuf.Bytes())
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates/import", &formBuf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST import failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on import, got %d", resp.StatusCode)
	}

	var importResp struct {
		Success   bool        `json:"success"`
		Imported  int         `json:"imported"`
		Templates []*Template `json:"templates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&importResp); err != nil {
		t.Fatalf("failed to decode import response: %v", err)
	}
	resp.Body.Close()

	if !importResp.Success {
		t.Errorf("expected success: true")
	}
	if importResp.Imported != 2 {
		t.Fatalf("expected 2 imported templates, got %d", importResp.Imported)
	}

	names := make(map[string]*Template)
	for _, tmpl := range importResp.Templates {
		names[tmpl.Name] = tmpl
		if tmpl.Serial != "dev-import" {
			t.Errorf("expected serial dev-import, got %s", tmpl.Serial)
		}
		if tmpl.Threshold != 0.88 {
			t.Errorf("expected threshold 0.88, got %f", tmpl.Threshold)
		}
		if !tmpl.Grayscale {
			t.Errorf("expected grayscale true")
		}
	}

	if names["confirm_ok"] == nil || names["cancel_button"] == nil {
		t.Errorf("expected confirm_ok and cancel_button in imported templates, got %+v", names)
	}

	// 3. Error cases for import
	// Missing file field
	var emptyForm bytes.Buffer
	mwEmpty := multipart.NewWriter(&emptyForm)
	_ = mwEmpty.WriteField("serial", "dev-import")
	_ = mwEmpty.Close()
	reqBad, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates/import", &emptyForm)
	reqBad.Header.Set("Content-Type", mwEmpty.FormDataContentType())
	respBad, _ := client.Do(reqBad)
	if respBad.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file field, got %d", respBad.StatusCode)
	}
	respBad.Body.Close()

	// Invalid zip file
	var badZipForm bytes.Buffer
	mwBadZip := multipart.NewWriter(&badZipForm)
	partBad, _ := mwBadZip.CreateFormFile("file", "corrupt.zip")
	_, _ = partBad.Write([]byte("not a zip file at all"))
	_ = mwBadZip.Close()
	reqBadZip, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates/import", &badZipForm)
	reqBadZip.Header.Set("Content-Type", mwBadZip.FormDataContentType())
	respBadZip, _ := client.Do(reqBadZip)
	if respBadZip.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for corrupt zip, got %d", respBadZip.StatusCode)
	}
	respBadZip.Body.Close()
}

func TestHandler_DownloadImage(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	// Create template with special characters and Unicode
	img := createDistinctiveTemplate(24, 24)
	b64 := encodeImageToBase64(img)
	createReq := CreateTemplateRequest{
		ID:          "tmpl_dl_test",
		Name:        "确认 按钮?/*",
		Serial:      "dev-dl",
		ImageBase64: b64,
	}
	bodyBytes, _ := json.Marshal(createReq)
	resp, err := client.Post(ts.URL+"/api/templates", "application/json", bytes.NewReader(bodyBytes))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create template: %v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 1. GET /api/templates/{id}/image?download=1
	resp, err = client.Get(ts.URL + "/api/templates/tmpl_dl_test/image?download=1")
	if err != nil {
		t.Fatalf("GET with download=1 failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected image/png, got %s", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	expectedCD := `attachment; filename="确认 按钮___.png"`
	if cd != expectedCD {
		t.Errorf("expected Content-Disposition %q, got %q", expectedCD, cd)
	}
	resp.Body.Close()

	// 2. GET /api/templates/{id}/image (without download=1)
	respNoDl, err := client.Get(ts.URL + "/api/templates/tmpl_dl_test/image")
	if err != nil {
		t.Fatalf("GET without download failed: %v", err)
	}
	if respNoDl.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respNoDl.StatusCode)
	}
	if cdNoDl := respNoDl.Header.Get("Content-Disposition"); cdNoDl != "" {
		t.Errorf("expected empty Content-Disposition without download=1, got %q", cdNoDl)
	}
	respNoDl.Body.Close()
}

func TestHandler_CreateTemplate_AutoNameFromFile(t *testing.T) {
	ts, _, dir := setupTestServer(t)
	defer ts.Close()
	defer os.RemoveAll(dir)

	client := ts.Client()

	img := createDistinctiveTemplate(20, 20)

	// 1. Name omitted/empty -> extract from filename "返回按钮.png"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("serial", "dev-auto")
	part, err := mw.CreateFormFile("file", "返回按钮.png")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	_ = png.Encode(part, img)
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST auto-name failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}
	var created1 Template
	_ = json.NewDecoder(resp.Body).Decode(&created1)
	resp.Body.Close()

	if created1.Name != "返回按钮" {
		t.Errorf("expected template name '返回按钮', got %q", created1.Name)
	}

	// 2. Explicit name provided -> use explicit name even if filename is different
	var buf2 bytes.Buffer
	mw2 := multipart.NewWriter(&buf2)
	_ = mw2.WriteField("name", "自定义名称")
	_ = mw2.WriteField("serial", "dev-auto")
	part2, err := mw2.CreateFormFile("image", "返回按钮.png")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	_ = png.Encode(part2, img)
	_ = mw2.Close()

	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/templates", &buf2)
	req2.Header.Set("Content-Type", mw2.FormDataContentType())

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("POST explicit name failed: %v", err)
	}
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp2.StatusCode)
	}
	var created2 Template
	_ = json.NewDecoder(resp2.Body).Decode(&created2)
	resp2.Body.Close()

	if created2.Name != "自定义名称" {
		t.Errorf("expected template name '自定义名称', got %q", created2.Name)
	}
}
