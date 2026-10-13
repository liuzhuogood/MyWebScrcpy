package dependencies

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	raw := http.MaxBytesReader(w, r.Body, 8192)
	defer raw.Close()
	d := json.NewDecoder(raw)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("请求格式错误")
	}
	return nil
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func registerManagedSettings(mux *http.ServeMux, authorize func(http.HandlerFunc) http.HandlerFunc) {
	if active == nil {
		return
	}
	m := active
	mux.HandleFunc("PUT /api/dependencies/config", authorize(func(w http.ResponseWriter, r *http.Request) {
		var s Settings
		if e := decode(w, r, &s); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		if e := m.Apply(s); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/dependencies/detect", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID        string    `json:"id"`
			Selection Selection `json:"selection"`
		}
		if e := decode(w, r, &body); e != nil || !knownID(body.ID) {
			http.Error(w, "依赖检测参数错误", 400)
			return
		}
		s := m.Settings()
		if body.ID == "adb" {
			s.ADB = body.Selection
		} else {
			s.Pi = body.Selection
		}
		if e := validateShape(s); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		release, e := m.acquire(body.ID)
		if e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		defer release()
		respond(w, m.detect(body.ID, body.Selection))
	}))
	mux.HandleFunc("POST /api/dependencies/install", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID            string `json:"id"`
			AcceptLicense bool   `json:"accept_license"`
		}
		if e := decode(w, r, &body); e != nil {
			http.Error(w, "安装参数错误", 400)
			return
		}
		if e := m.Install(body.ID, body.AcceptLicense); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		w.WriteHeader(202)
	}))
	mux.HandleFunc("GET /api/dependencies/install/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !knownID(id) {
			http.NotFound(w, r)
			return
		}
		m.mu.RLock()
		task := m.tasks[id]
		m.mu.RUnlock()
		respond(w, task)
	})
	mux.HandleFunc("DELETE /api/dependencies/install/{id}", authorize(func(w http.ResponseWriter, r *http.Request) {
		if e := m.Cancel(r.PathValue("id")); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("DELETE /api/dependencies/installed/{id}", authorize(func(w http.ResponseWriter, r *http.Request) {
		if e := m.Uninstall(r.PathValue("id")); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		w.WriteHeader(204)
	}))
}
