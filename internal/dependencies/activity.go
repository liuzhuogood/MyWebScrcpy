package dependencies

import (
	"context"
	"sync"
)

type backgroundADBKey struct{}

// BackgroundADB identifies passive monitoring and power management, not user work.
func BackgroundADB(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundADBKey{}, true)
}

var deviceActivity struct {
	sync.RWMutex
	begin func(string) func()
}

// SetDeviceActivityObserver is configured once before serving requests.
func SetDeviceActivityObserver(begin func(string) func()) {
	deviceActivity.Lock()
	deviceActivity.begin = begin
	deviceActivity.Unlock()
}

func BeginDeviceActivity(serial string) func() {
	deviceActivity.RLock()
	begin := deviceActivity.begin
	deviceActivity.RUnlock()
	if serial == "" || begin == nil {
		return func() {}
	}
	return begin(serial)
}

func commandSerial(args []string) string {
	if len(args) >= 2 && args[0] == "-s" {
		return args[1]
	}
	if len(args) == 2 && (args[0] == "connect" || args[0] == "disconnect") {
		return args[1]
	}
	return ""
}
