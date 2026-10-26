package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/device"
	"mywebscrcpy/internal/idlescreen"
)

func configureIdleScreen(database *db.DB, dm *device.Manager, adbPath string) *idlescreen.Manager {
	var store idlescreen.Store
	if database != nil {
		store = database
	}
	return idlescreen.New(store, func() []string {
		var serials []string
		for _, d := range dm.Devices() {
			if d.State == "device" {
				serials = append(serials, d.Serial)
			}
		}
		return serials
	}, func(ctx context.Context, serial string) error {
		return sleepIdleDevice(ctx, adbPath, serial)
	})
}

func sleepIdleDevice(ctx context.Context, adbPath, serial string) error {
	ctx = dependencies.BackgroundADB(ctx)
	if out, err := dependencies.CommandContext(ctx, adbPath, "-s", serial, "shell", "input", "keyevent", "223").CombinedOutput(); err != nil {
		return fmt.Errorf("发送熄屏命令: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// Power state changes asynchronously; confirm the result instead of trusting exit 0.
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		out, err := dependencies.CommandContext(ctx, adbPath, "-s", serial, "shell", "dumpsys", "power").Output()
		if err != nil {
			return fmt.Errorf("检查熄屏状态: %w", err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "mWakefulness=Asleep" || line == "mWakefulness=Dozing" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("手机未进入休眠: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// Holds a device lease for the whole request, including websocket and multi-step work.
func deviceActivityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/screen-state" || r.URL.Path == "/api/vision/frame" {
			next.ServeHTTP(w, r)
			return
		}
		serial := r.URL.Query().Get("serial")
		// These routes declare serial as their first path parameter in their schemas.
		for _, prefix := range []string{"/api/device-state/", "/api/devices/"} {
			if strings.HasPrefix(r.URL.Path, prefix) {
				serial = strings.SplitN(strings.TrimPrefix(r.URL.Path, prefix), "/", 2)[0]
				break
			}
		}
		if serial != "" {
			defer dependencies.BeginDeviceActivity(serial)()
		}
		next.ServeHTTP(w, r)
	})
}
