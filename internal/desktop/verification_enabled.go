//go:build desktopverify

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
)

// Only compiled into the explicit native verification build. Reports omit
// device/user data and the private control token.
func registerVerification(mux *http.ServeMux, authorize func(http.HandlerFunc) http.HandlerFunc) {
	path := os.Getenv("MYWEBSCRCPY_NATIVE_REPORT")
	if path == "" {
		return
	}
	mux.HandleFunc("POST /api/desktop/native-verification", authorize(func(w http.ResponseWriter, r *http.Request) {
		var report struct {
			CloseHides        bool   `json:"close_hides"`
			RestoreVisible    bool   `json:"restore_visible"`
			Title             string `json:"title"`
			Mounted           bool   `json:"mounted"`
			VideoDecoder      bool   `json:"video_decoder"`
			AudioDecoder      bool   `json:"audio_decoder"`
			Path              string `json:"path"`
			DependencyHeading bool   `json:"dependency_heading"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&report); err != nil {
			http.Error(w, "invalid report", 400)
			return
		}
		bytes, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			http.Error(w, "invalid report", 400)
			return
		}
		if err = os.WriteFile(path, bytes, 0600); err != nil {
			http.Error(w, "report failed", 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
