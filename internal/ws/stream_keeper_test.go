package ws

import (
	"context"
	"testing"
	"time"

	"mywebscrcpy/internal/visionframes"
)

func TestStreamKeeperLifecycle(t *testing.T) {
	frames := visionframes.NewStore()
	h := &Hub{
		sessions: make(map[string]*managedSession),
	}
	keeper := NewStreamKeeper(h, frames)
	defer keeper.Close()

	serial := "test-device-1"

	if keeper.IsActive(serial) {
		t.Fatalf("expected inactive initially")
	}

	// 存入一帧画面
	testJPEG := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xdb\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d\x1a\x1c\x1c $.' \",#\x1c\x1c(7),01444\x1f'9=82<.342\xff\xc0\x00\x0b\x08\x00\x02\x00\x02\x01\x01\x11\x00\xff\xda\x00\x08\x01\x01\x00\x00?\x00\xbf\x00\xff\xd9")
	if _, err := frames.Put(serial, testJPEG); err != nil {
		t.Fatalf("Put test frame failed: %v", err)
	}

	// 模拟已存在 managedSession
	ms := &managedSession{
		h:    h,
		meta: &handshakeMeta{SessionID: "sess-1"},
		subs: make(map[chan sharedVideoFrame]struct{}),
	}
	h.sessions[serial] = ms

	// 测试 Touch
	if err := keeper.Touch(serial, 200*time.Millisecond); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	if !keeper.IsActive(serial) {
		t.Fatalf("expected active after Touch")
	}

	active := keeper.ActiveSerials()
	if len(active) != 1 || active[0] != serial {
		t.Fatalf("unexpected active serials: %v", active)
	}

	// 测试 EnsureFrame 能立即命中，并自动延时保活时长
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	f, err := keeper.EnsureFrame(ctx, serial, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("EnsureFrame failed: %v", err)
	}
	if f.Width != 2 || f.Height != 2 {
		t.Fatalf("unexpected frame dimensions: %dx%d", f.Width, f.Height)
	}

	// EnsureFrame 自动重置延时，因此依然活跃
	if !keeper.IsActive(serial) {
		t.Fatalf("expected active after EnsureFrame refreshed TTL")
	}

	// 重新设置较短的 150ms 租约测试到期释放
	_ = keeper.Touch(serial, 150*time.Millisecond)
	time.Sleep(250 * time.Millisecond)
	if keeper.IsActive(serial) {
		t.Fatalf("expected lease to expire after timeout")
	}
}
