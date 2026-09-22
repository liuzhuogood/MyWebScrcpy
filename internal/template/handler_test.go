package template

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTemplateManagementServer(t *testing.T) *httptest.Server {
	t.Helper()
	storage, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	service := NewService(storage)
	t.Cleanup(service.Stop)
	mux := http.NewServeMux()
	RegisterRoutes(mux, service)
	return httptest.NewServer(mux)
}

func managementPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}

func TestTemplateAssetCRUD(t *testing.T) {
	ts := newTemplateManagementServer(t)
	defer ts.Close()

	request := CreateTemplateRequest{
		Name:        "confirm",
		Serial:      "device-1",
		ImageBase64: "data:image/png;base64," + base64.StdEncoding.EncodeToString(managementPNG(t)),
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(ts.URL+"/api/templates", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d, want %d", response.StatusCode, http.StatusCreated)
	}
	var created Template
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	response, err = http.Get(ts.URL + "/api/templates?serial=device-1")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Templates []*Template `json:"templates"`
	}
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if len(list.Templates) != 1 || list.Templates[0].ID != created.ID {
		t.Fatalf("listed templates=%+v, want created template", list.Templates)
	}

	response, err = http.Get(ts.URL + "/api/templates/" + created.ID + "/image")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("image response status=%d content-type=%q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	_ = response.Body.Close()

	deleteRequest, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/templates/"+created.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err = http.DefaultClient.Do(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d, want %d", response.StatusCode, http.StatusOK)
	}
	_ = response.Body.Close()
}

func TestTemplateMatchingRoutesAreNotRegistered(t *testing.T) {
	ts := newTemplateManagementServer(t)
	defer ts.Close()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/templates/status?serial=device-1"},
		{http.MethodGet, "/api/templates/matches?serial=device-1"},
		{http.MethodPost, "/api/templates/detect"},
		{http.MethodPost, "/api/templates/click"},
	} {
		req, err := http.NewRequest(tc.method, ts.URL+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices {
			t.Fatalf("%s %s unexpectedly remains available: status=%d", tc.method, tc.path, res.StatusCode)
		}
		_ = res.Body.Close()
	}
}
