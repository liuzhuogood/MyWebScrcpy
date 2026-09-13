package uipage

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestParseComponent(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantPkg   string
		wantAct   string
		wantComp  string
		wantOk    bool
	}{
		{
			name:     "standard package and activity",
			raw:      "com.uutalk.im/com.jiangxia.im.MainActivity",
			wantPkg:  "com.uutalk.im",
			wantAct:  "com.jiangxia.im.MainActivity",
			wantComp: "com.uutalk.im/com.jiangxia.im.MainActivity",
			wantOk:   true,
		},
		{
			name:     "short sub activity",
			raw:      "com.example.app/.SubActivity",
			wantPkg:  "com.example.app",
			wantAct:  "com.example.app.SubActivity",
			wantComp: "com.example.app/com.example.app.SubActivity",
			wantOk:   true,
		},
		{
			name:     "invalid single word",
			raw:      "invalid_component",
			wantOk:   false,
		},
		{
			name:     "null string",
			raw:      "null",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, act, comp, ok := normalizeComponent(tt.raw)
			if ok != tt.wantOk {
				t.Fatalf("normalizeComponent(%q) ok = %v, want %v", tt.raw, ok, tt.wantOk)
			}
			if ok {
				if pkg != tt.wantPkg || act != tt.wantAct || comp != tt.wantComp {
					t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", pkg, act, comp, tt.wantPkg, tt.wantAct, tt.wantComp)
				}
			}
		})
	}
}

func TestFetchPageInfoFromWindow(t *testing.T) {
	windowDump := `
WINDOW MANAGER DUMP
  mCurrentFocus=Window{a833e82 u0 com.uutalk.im/com.jiangxia.im.MainActivity}
  mFocusedApp=AppWindowToken{379d479 token=Token{2fe5b70 ActivityRecord{a43f2cb u0 com.uutalk.im/com.jiangxia.im.MainActivity t123}}}
`
	wmSize := `Physical size: 1080x2400
Override size: 720x1600`
	wmDensity := `Physical density: 420
Override density: 320`
	inputDump := `
Input Dispatcher State:
  SurfaceOrientation: 1
`

	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		cmd := stringsJoin(args)
		switch {
		case containsAll(cmd, "dumpsys", "window"):
			return []byte(windowDump), nil
		case containsAll(cmd, "wm", "size"):
			return []byte(wmSize), nil
		case containsAll(cmd, "wm", "density"):
			return []byte(wmDensity), nil
		case containsAll(cmd, "dumpsys", "input"):
			return []byte(inputDump), nil
		default:
			return nil, errors.New("unknown command")
		}
	})

	info, err := s.FetchPageInfo(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("FetchPageInfo failed: %v", err)
	}

	if info.PackageName != "com.uutalk.im" {
		t.Errorf("PackageName = %s, want com.uutalk.im", info.PackageName)
	}
	if info.Activity != "com.jiangxia.im.MainActivity" {
		t.Errorf("Activity = %s, want com.jiangxia.im.MainActivity", info.Activity)
	}
	if info.Component != "com.uutalk.im/com.jiangxia.im.MainActivity" {
		t.Errorf("Component = %s", info.Component)
	}
	if info.Display == nil {
		t.Fatalf("expected Display to be non-nil")
	}
	if info.Display.Width != 720 || info.Display.Height != 1600 {
		t.Errorf("Display size = %dx%d, want 720x1600 (override)", info.Display.Width, info.Display.Height)
	}
	if info.Display.DensityDpi != 320 {
		t.Errorf("Display density = %d, want 320 (override)", info.Display.DensityDpi)
	}
	if info.Display.Rotation != 90 {
		t.Errorf("Display rotation = %d, want 90", info.Display.Rotation)
	}
}

func TestFetchPageInfoFallbackActivity(t *testing.T) {
	windowDump := `
WINDOW MANAGER DUMP
  mCurrentFocus=null
  mFocusedApp=null
`
	actDump := `
ACTIVITY MANAGER ACTIVITIES (dumpsys activity activities)
  Display #0 (activities from top to bottom):
    Stack #1:
      Task id #123
        mResumedActivity: ActivityRecord{8b196fa u0 com.android.settings/.Settings t123}
`
	wmSize := `Physical size: 1080x1920`
	wmDensity := `Physical density: 480`
	winDisplaysDump := `
WINDOW MANAGER DISPLAY
  Display: mDisplayId=0
    init=1080x1920 480dpi cur=1080x1920 app=1080x1920 rng=1080x1000
    mCurrentRotation=3
`

	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		cmd := stringsJoin(args)
		switch {
		case containsAll(cmd, "dumpsys", "window", "displays"):
			return []byte(winDisplaysDump), nil
		case containsAll(cmd, "dumpsys", "window"):
			return []byte(windowDump), nil
		case containsAll(cmd, "dumpsys", "activity", "activities"):
			return []byte(actDump), nil
		case containsAll(cmd, "wm", "size"):
			return []byte(wmSize), nil
		case containsAll(cmd, "wm", "density"):
			return []byte(wmDensity), nil
		case containsAll(cmd, "dumpsys", "input"):
			return nil, errors.New("input dumpsys unavailable")
		default:
			return nil, errors.New("unknown command")
		}
	})

	info, err := s.FetchPageInfo(context.Background(), "dev-2")
	if err != nil {
		t.Fatalf("FetchPageInfo failed: %v", err)
	}

	if info.PackageName != "com.android.settings" {
		t.Errorf("PackageName = %s, want com.android.settings", info.PackageName)
	}
	if info.Activity != "com.android.settings.Settings" {
		t.Errorf("Activity = %s, want com.android.settings.Settings", info.Activity)
	}
	if info.Component != "com.android.settings/com.android.settings.Settings" {
		t.Errorf("Component = %s", info.Component)
	}
	if info.Display == nil {
		t.Fatalf("expected Display to be non-nil")
	}
	if info.Display.Width != 1080 || info.Display.Height != 1920 {
		t.Errorf("Display size = %dx%d, want 1080x1920", info.Display.Width, info.Display.Height)
	}
	if info.Display.DensityDpi != 480 {
		t.Errorf("Display density = %d, want 480", info.Display.DensityDpi)
	}
	if info.Display.Rotation != 270 {
		t.Errorf("Display rotation = %d, want 270", info.Display.Rotation)
	}
}

func TestFetchPageInfoNotFound(t *testing.T) {
	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("empty dump"), nil
	})
	_, err := s.FetchPageInfo(context.Background(), "dev-none")
	if !errors.Is(err, ErrPageInfoNotFound) {
		t.Fatalf("expected ErrPageInfoNotFound, got: %v", err)
	}
}

func TestFetchPageInfoMissingSerial(t *testing.T) {
	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		return []byte(""), nil
	})
	_, err := s.FetchPageInfo(context.Background(), "   ")
	if !errors.Is(err, ErrMissingSerial) {
		t.Fatalf("expected ErrMissingSerial, got: %v", err)
	}
}

func TestFetchPageInfoConcurrencyLock(t *testing.T) {
	var mu sync.Mutex
	active, maxActive := 0, 0

	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()

		return []byte("mCurrentFocus=Window{123 u0 com.example/.Main}"), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.FetchPageInfo(context.Background(), "dev-locked")
		}()
	}
	wg.Wait()

	if maxActive != 1 {
		t.Fatalf("expected max concurrency per serial to be 1, got %d", maxActive)
	}
}

func stringsJoin(args []string) string {
	res := ""
	for _, a := range args {
		res += " " + a
	}
	return res
}

func containsAll(str string, subs ...string) bool {
	for _, sub := range subs {
		if !reflect.ValueOf(str).MethodByName("Contains").IsValid() {
			// fallback check
		}
		if !contains(str, sub) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
