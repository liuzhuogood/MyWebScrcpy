package scrcpy

import (
	"encoding/binary"
	"testing"
)

func TestTextEvent(t *testing.T) {
	b := TextEvent("你好")
	if len(b) != 11 || b[0] != TypeInjectText {
		t.Fatalf("unexpected text event length/type: %d %#v", len(b), b)
	}
	if got := binary.BigEndian.Uint32(b[1:5]); got != uint32(len("你好")) {
		t.Fatalf("unexpected UTF-8 byte length: %d", got)
	}
	if string(b[5:]) != "你好" {
		t.Fatalf("unexpected text payload: %q", b[5:])
	}
}

func TestKeyCodeEvent(t *testing.T) {
	b := KeyCodeEvent(KeyActionDown, 4, 2, 3)
	if len(b) != 14 || b[0] != TypeInjectKeyCode || b[1] != KeyActionDown {
		t.Fatalf("unexpected key event: %#v", b)
	}
	if binary.BigEndian.Uint32(b[2:6]) != 4 || binary.BigEndian.Uint32(b[6:10]) != 2 || binary.BigEndian.Uint32(b[10:14]) != 3 {
		t.Fatalf("unexpected key fields: %#v", b)
	}
}

func TestTouchEvent(t *testing.T) {
	b := TouchEvent(ActionMove, PointerIDMouse, 12, 34, 100, 200, 1, ButtonPrimary, ButtonPrimary)
	if len(b) != 32 || b[0] != TypeInjectTouchEvent || b[1] != ActionMove {
		t.Fatalf("unexpected touch event: %#v", b)
	}
	if binary.BigEndian.Uint32(b[10:14]) != 12 || binary.BigEndian.Uint32(b[14:18]) != 34 || binary.BigEndian.Uint16(b[18:20]) != 100 || binary.BigEndian.Uint16(b[20:22]) != 200 {
		t.Fatalf("unexpected touch fields: %#v", b)
	}
}
