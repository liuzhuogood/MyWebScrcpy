package desktop

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestControlOwnership(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	control.Register(mux)
	for _, token := range []string{"", "wrong"} {
		req := httptest.NewRequest("POST", "/api/desktop/shutdown", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != 401 {
			t.Fatalf("unowned shutdown status=%d", res.Code)
		}
		select {
		case <-control.Done():
			t.Fatal("unowned shutdown stopped service")
		default:
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(t.TempDir(), "ready.json")
	if err := control.Publish(path, listener); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ready Ready
	if err = json.Unmarshal(contents, &ready); err != nil {
		t.Fatal(err)
	}
	if ready.URL != "http://"+listener.Addr().String() || ready.PID != os.Getpid() || len(ready.Token) != 64 {
		t.Fatalf("bad readiness: %+v", ready)
	}
	req := httptest.NewRequest("POST", "/api/desktop/shutdown", nil)
	req.Header.Set("Authorization", "Bearer "+ready.Token)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != 202 {
		t.Fatal(res.Code)
	}
	select {
	case <-control.Done():
	default:
		t.Fatal("owned shutdown did not stop")
	}
}
