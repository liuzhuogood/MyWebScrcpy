package ws

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/scrcpy"
)

var browserControlSequence atomic.Uint64

const maxOwnedControlInputs = 256

type ownedControlInput struct {
	owner string
	raw   []byte
}

// controlInputOwners is protected by managedSession.controlMu, alongside the
// corresponding socket write. Reusing a mouse ID cannot let an old connection
// release the input subsequently taken over by another connection.
type controlInputOwners struct {
	ownersMu sync.RWMutex
	owners   map[string]int
	touches  map[uint64]ownedControlInput
	keys     map[uint32]ownedControlInput
	back     *ownedControlInput
}

// Ownership counts are separately readable while another SDK/HID writer holds
// controlMu. An idle browser's disconnect must never close that healthy session.
func (s *controlInputOwners) hasOwner(owner string) bool {
	s.ownersMu.RLock()
	defer s.ownersMu.RUnlock()
	return s.owners[owner] > 0
}

func (s *controlInputOwners) transfer(previous, current string) {
	if previous == current {
		return
	}
	s.ownersMu.Lock()
	defer s.ownersMu.Unlock()
	if previous != "" {
		s.owners[previous]--
		if s.owners[previous] <= 0 {
			delete(s.owners, previous)
		}
	}
	if current != "" {
		if s.owners == nil {
			s.owners = make(map[string]int)
		}
		s.owners[current]++
	}
}

type controlWriter func(context.Context, []byte) error

func (s *controlInputOwners) write(ctx context.Context, owner string, raw []byte, write controlWriter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if owner == "" {
		return write(ctx, raw)
	}
	if len(raw) == 32 && raw[0] == scrcpy.TypeInjectTouchEvent {
		id := binary.BigEndian.Uint64(raw[2:10])
		previous, held := s.touches[id]
		switch raw[1] {
		case scrcpy.ActionMove, scrcpy.ActionUp:
			if held && previous.owner != owner {
				return nil
			}
		case 3: // Android ACTION_CANCEL cancels the gesture, not another client's keys.
			return s.release(ctx, owner, true, write)
		case scrcpy.ActionDown:
			if !held && len(s.touches)+len(s.keys) >= maxOwnedControlInputs {
				return errors.New("too_many_held_inputs")
			}
		}
		if err := write(ctx, raw); err != nil {
			return err
		}
		switch raw[1] {
		case scrcpy.ActionDown, scrcpy.ActionMove:
			if raw[1] == scrcpy.ActionDown || held {
				if s.touches == nil {
					s.touches = make(map[uint64]ownedControlInput)
				}
				s.transfer(previous.owner, owner)
				s.touches[id] = ownedControlInput{owner, append([]byte(nil), raw...)}
			}
		case scrcpy.ActionUp:
			if held {
				s.transfer(previous.owner, "")
			}
			delete(s.touches, id)
		}
		return nil
	}
	if len(raw) == 14 && raw[0] == scrcpy.TypeInjectKeyCode {
		code := binary.BigEndian.Uint32(raw[2:6])
		previous, held := s.keys[code]
		if raw[1] == scrcpy.KeyActionUp && held && previous.owner != owner {
			return nil
		}
		if raw[1] == scrcpy.KeyActionDown && !held && len(s.touches)+len(s.keys) >= maxOwnedControlInputs {
			return errors.New("too_many_held_inputs")
		}
		if err := write(ctx, raw); err != nil {
			return err
		}
		if raw[1] == scrcpy.KeyActionDown {
			if s.keys == nil {
				s.keys = make(map[uint32]ownedControlInput)
			}
			s.transfer(previous.owner, owner)
			s.keys[code] = ownedControlInput{owner, append([]byte(nil), raw...)}
		} else if raw[1] == scrcpy.KeyActionUp {
			if held {
				s.transfer(previous.owner, "")
			}
			delete(s.keys, code)
		}
		return nil
	}
	if len(raw) == 2 && raw[0] == scrcpy.TypeBackOrScreenOn {
		if raw[1] == scrcpy.KeyActionUp && s.back != nil && s.back.owner != owner {
			return nil
		}
		if err := write(ctx, raw); err != nil {
			return err
		}
		if raw[1] == scrcpy.KeyActionDown {
			previous := ""
			if s.back != nil {
				previous = s.back.owner
			}
			s.transfer(previous, owner)
			s.back = &ownedControlInput{owner, append([]byte(nil), raw...)}
		} else if raw[1] == scrcpy.KeyActionUp {
			if s.back != nil {
				s.transfer(s.back.owner, "")
			}
			s.back = nil
		}
		return nil
	}
	return write(ctx, raw)
}

func (s *controlInputOwners) release(ctx context.Context, owner string, touchesOnly bool, write controlWriter) error {
	for id, input := range s.touches {
		if input.owner != owner {
			continue
		}
		raw := append([]byte(nil), input.raw...)
		raw[1] = scrcpy.ActionUp
		clear(raw[22:32]) // Release pressure and every mouse button.
		if err := write(ctx, raw); err != nil {
			return err
		}
		s.transfer(input.owner, "")
		delete(s.touches, id)
	}
	if touchesOnly {
		return nil
	}
	for code, input := range s.keys {
		if input.owner != owner {
			continue
		}
		raw := append([]byte(nil), input.raw...)
		raw[1] = scrcpy.KeyActionUp
		clear(raw[6:14]) // A synthetic release is neither a repeat nor a modifier press.
		if err := write(ctx, raw); err != nil {
			return err
		}
		s.transfer(input.owner, "")
		delete(s.keys, code)
	}
	if s.back != nil && s.back.owner == owner {
		if err := write(ctx, []byte{scrcpy.TypeBackOrScreenOn, scrcpy.KeyActionUp}); err != nil {
			return err
		}
		s.transfer(s.back.owner, "")
		s.back = nil
	}
	return nil
}

// Caller owns controlMu. All raw SDK clients retain their existing wire format.
func (ms *managedSession) writeBrowserControl(ctx context.Context, r action.Request) error {
	write := ms.sess.conn.WriteControlContext
	return ms.controlInputs.write(ctx, r.ControlOwner, r.Raw, write)
}

// Disconnect cleanup must not wait behind unrelated ADB jobs in the action gate.
// The control mutex still serializes releases with every actual SDK/HID write.
func (ms *managedSession) releaseBrowserControl(owner string) {
	if ms.sess == nil || ms.sess.conn == nil || !ms.controlInputs.hasOwner(owner) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for !ms.controlMu.TryLock() {
		if !ms.controlInputs.hasOwner(owner) {
			return
		}
		select {
		case <-ctx.Done():
			if !ms.controlInputs.hasOwner(owner) {
				return
			}
			log.Printf("[ws] control release timed out serial=%s owner=%s; closing stalled session", ms.meta.Serial, owner)
			ms.sess.conn.Close()
			return
		case <-ticker.C:
		}
	}
	defer ms.controlMu.Unlock()
	if err := ms.controlInputs.release(ctx, owner, false, ms.sess.conn.WriteControlContext); err != nil {
		log.Printf("[ws] control release failed serial=%s owner=%s: %v; closing stalled session", ms.meta.Serial, owner, err)
		ms.sess.conn.Close()
	}
}
