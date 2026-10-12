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
