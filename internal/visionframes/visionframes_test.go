package visionframes

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPutAndLatest(t *testing.T) {
	s := NewStore()
	if _, ok := s.Latest("dev", time.Second); ok {
		t.Fatal("empty store should miss")
	}
	if _, err := s.Put("dev", testJPEG(t, 448, 1024)); err != nil {
		t.Fatal(err)
	}
	f, ok := s.Latest("dev", time.Second)
	if !ok || f.Width != 448 || f.Height != 1024 || len(f.JPEG) == 0 {
		t.Fatalf("latest frame wrong: %+v", f)
	}
	// 过期后不可取
	time.Sleep(10 * time.Millisecond)
	if _, ok := s.Latest("dev", 5*time.Millisecond); ok {
		t.Fatal("expired frame should miss")
	}
}

func TestPutRejectsNonJPEG(t *testing.T) {
	s := NewStore()
	if _, err := s.Put("dev", []byte("not an image")); err == nil {
		t.Fatal("non-JPEG payload should be rejected")
	}
}

func TestUploadHandler(t *testing.T) {
	s := NewStore()
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/api/vision/frame?serial=dev-1", bytes.NewReader(testJPEG(t, 10, 20)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}
	if _, ok := s.Latest("dev-1", time.Second); !ok {
		t.Fatal("frame not stored after upload")
	}

	// 缺 serial
	req = httptest.NewRequest("POST", "/api/vision/frame", bytes.NewReader(testJPEG(t, 10, 20)))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("missing serial should 400, got %d", w.Code)
	}

	// 非法数据
	req = httptest.NewRequest("POST", "/api/vision/frame?serial=dev-1", strings.NewReader("junk"))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("junk payload should 400, got %d", w.Code)
	}
}
