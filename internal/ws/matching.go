package ws

import (
	"fmt"
	"sort"
)

// MatchingDevices returns live sessions with references other than the matching service.
func (h *Hub) MatchingDevices() []string {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	result := []string{}
	for serial, ms := range h.sessions {
		ms.mu.Lock()
		active := !ms.replay && ms.sess != nil && ms.refs > ms.matchingRefs
		ms.mu.Unlock()
		if active {
			result = append(result, serial)
		}
	}
	sort.Strings(result)
	return result
}

// Acquire under the registry lock: a player disappearing must never start a phone.
func (h *Hub) acquireMatchingSession(serial string) (*managedSession, *handshakeMeta, func(), error) {
	h.sessionMu.Lock()
	ms := h.sessions[serial]
	if h.closed || ms == nil {
		h.sessionMu.Unlock()
		return nil, nil, nil, fmt.Errorf("投屏会话已关闭")
	}
	ms.mu.Lock()
	keeperActive := h.streamKeeper != nil && h.streamKeeper.IsActive(serial)
	if ms.replay || ms.sess == nil || (!keeperActive && ms.refs <= ms.matchingRefs) {
		ms.mu.Unlock()
		h.sessionMu.Unlock()
		return nil, nil, nil, fmt.Errorf("没有实时投屏会话")
	}
	ms.refs++
	ms.matchingRefs++
	if ms.closeTimer != nil {
		ms.closeTimer.Stop()
		ms.closeTimer = nil
	}
	ms.mu.Unlock()
	h.sessionMu.Unlock()
	return ms, ms.meta, func() { ms.mu.Lock(); ms.matchingRefs--; ms.mu.Unlock(); h.releaseSession(serial, ms) }, nil
}
