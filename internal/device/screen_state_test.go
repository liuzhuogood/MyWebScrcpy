package device

import (
	"context"
	"strings"
	"testing"
)

func TestReadScreenStateUsesADBPowerAndPolicy(t *testing.T) {
	t.Setenv("FAKE_ADB_MODE", "screen")
	t.Setenv("FAKE_ADB_POWER", "mWakefulness=Awake")
	t.Setenv("FAKE_ADB_POLICY", "showing=true")
	m := NewManager(buildFakeADB(t), nil)
	state := m.ReadScreenState(context.Background(), "fake-device", true)
	if state.Error != "" || state.ScreenOn == nil || !*state.ScreenOn || state.ScreenLocked == nil || !*state.ScreenLocked {
		t.Fatalf("unexpected state: %#v", state)
	}

	t.Setenv("FAKE_ADB_POWER", "mWakefulness=Dozing")
	state = m.ReadScreenState(context.Background(), "fake-device", false)
	if state.Error != "" || state.ScreenOn == nil || *state.ScreenOn || state.ScreenLocked != nil {
		t.Fatalf("off state without lock request should leave lock state unknown: %#v", state)
	}

	t.Setenv("FAKE_ADB_POWER", "unrecognized power output")
	state = m.ReadScreenState(context.Background(), "fake-device", false)
	if state.ScreenOn != nil || !strings.Contains(state.Error, "未返回可识别") {
		t.Fatalf("unknown power state: %#v", state)
	}
}

func TestReadScreenStateReportsADBCommandFailure(t *testing.T) {
	t.Setenv("FAKE_ADB_MODE", "fail")
	state := NewManager(buildFakeADB(t), nil).ReadScreenState(context.Background(), "fake-device", false)
	if state.ScreenOn != nil || !strings.Contains(state.Error, "读取屏幕电源状态失败") {
		t.Fatalf("expected power command failure: %#v", state)
	}
}
