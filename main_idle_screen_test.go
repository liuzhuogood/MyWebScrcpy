package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/dependencies"
)

func TestDeviceActivityRequestCoverage(t *testing.T) {
	for _, tc := range []struct{ url, serial string }{
		{"/ws?serial=phone", "phone"},
		{"/ws/adb-shell?serial=phone", "phone"},
		{"/api/adb/commands?serial=phone", "phone"},
		{"/api/device-state/phone?include_ocr=false", "phone"},
		{"/api/devices/phone/latest-frame", "phone"},
		{"/api/screen-state?serial=phone", ""},
		{"/api/vision/frame?serial=phone", ""},
		{"/api/devices", ""},
	} {
		t.Run(tc.url, func(t *testing.T) {
			active := ""
			released := false
			dependencies.SetDeviceActivityObserver(func(serial string) func() { active = serial; return func() { released = true } })
			defer dependencies.SetDeviceActivityObserver(nil)
			h := deviceActivityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if active != tc.serial || released {
					t.Fatalf("incorrect request lease: %s released=%t", active, released)
				}
			}))
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tc.url, nil))
			if tc.serial != "" && !released {
				t.Fatal("request lease leaked")
			}
		})
	}
}

// Opt-in integration: exercise the production sleep command and its real power-state check.
func TestIdleSleepRealDevices(t *testing.T) {
	serials := os.Getenv("MYWEBSCRCPY_IDLE_TEST_SERIALS")
	if serials == "" {
		t.Skip("real-device test is opt-in")
	}
	for _, serial := range strings.Split(serials, ",") {
		t.Run(serial, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "input", "keyevent", "224").Run(); err != nil {
				t.Fatal(err)
			}
			before, err := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "dumpsys", "power").Output()
			if err != nil || !strings.Contains(string(before), "mWakefulness=Awake") {
				t.Fatalf("device did not wake: %v", err)
			}
			if err := sleepIdleDevice(ctx, "adb", serial); err != nil {
				t.Fatal(err)
			}
		})
	}
}
