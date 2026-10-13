package ws

import "fmt"

// CheckUpdateIdle is called while the HTTP gate prevents new recording/replay requests.
func (h *Hub) CheckUpdateIdle() error {
	h.recordings.mu.Lock()
	recording := len(h.recordings.bySerial) > 0
	h.recordings.mu.Unlock()
	if recording {
		return fmt.Errorf("请停止录制后再更新")
	}
	h.replays.mu.Lock()
	replay := len(h.replays.bySerial) > 0
	h.replays.mu.Unlock()
	if replay {
		return fmt.Errorf("请停止回放后再更新")
	}
	return nil
}

// CheckDependencyIdle rejects changing ADB while sessions retain forwards and
// cached device metadata, including sessions waiting for their idle timeout.
func (h *Hub) CheckDependencyIdle() error {
	if err := h.CheckUpdateIdle(); err != nil {
		return err
	}
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	if len(h.sessions) > 0 || len(h.inflight) > 0 {
		return fmt.Errorf("请关闭投屏并等待会话释放后再修改 ADB")
	}
	h.envMu.Lock()
	h.deviceEnvs = make(map[string]deviceEnv)
	h.envMu.Unlock()
	return nil
}
