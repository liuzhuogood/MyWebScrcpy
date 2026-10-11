package scrcpy

import (
	"encoding/binary"
	"net"
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

func TestClipboardEvents(t *testing.T) {
	get := GetClipboardEvent(CopyKeyCopy)
	if len(get) != 2 || get[0] != TypeGetClipboard || get[1] != CopyKeyCopy {
		t.Fatalf("unexpected get clipboard event: %#v", get)
	}

	set := SetClipboardEvent("剪贴板测试", true, 42)
	if len(set) != 14+len("剪贴板测试") || set[0] != TypeSetClipboard {
		t.Fatalf("unexpected set clipboard length/type: %d %#v", len(set), set)
	}
	if binary.BigEndian.Uint64(set[1:9]) != 42 || set[9] != 1 {
		t.Fatalf("unexpected set clipboard sequence/paste: %#v", set[:10])
	}
	if binary.BigEndian.Uint32(set[10:14]) != uint32(len("剪贴板测试")) || string(set[14:]) != "剪贴板测试" {
		t.Fatalf("unexpected set clipboard payload: %q", set[14:])
	}
}

func TestReadDeviceMessage(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := &Connection{ctrlConn: client, hasControl: true}

	go func() {
		text := []byte("来自手机的文本")
		pkt := make([]byte, 5+len(text))
		pkt[0] = DeviceMsgClipboard
		binary.BigEndian.PutUint32(pkt[1:5], uint32(len(text)))
		copy(pkt[5:], text)
		_, _ = server.Write(pkt)

		ack := make([]byte, 9)
		ack[0] = DeviceMsgAckClipboard
		binary.BigEndian.PutUint64(ack[1:9], 99)
		_, _ = server.Write(ack)
	}()

	msg1, err := conn.ReadDeviceMessage()
	if err != nil || msg1.Type != DeviceMsgClipboard || msg1.Text != "来自手机的文本" {
		t.Fatalf("unexpected clipboard device message: %+v, err=%v", msg1, err)
	}

	msg2, err := conn.ReadDeviceMessage()
	if err != nil || msg2.Type != DeviceMsgAckClipboard || msg2.Sequence != 99 {
		t.Fatalf("unexpected ack device message: %+v, err=%v", msg2, err)
	}
}
