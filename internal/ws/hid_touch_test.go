package ws

import (
	"bytes"
	"testing"

	"mywebscrcpy/internal/scrcpy"
)

func TestUhidCreateLayout(t *testing.T) {
	desc := []byte{0x05, 0x0D, 0xC0}
	msg := scrcpy.UhidCreate(scrcpy.HIDIDTouch, 0x18D1, 0xFFFF, "scrcpy-touch", desc)
	// 1(type) + 2(id) + 2(vid) + 2(pid) + 1(nameLen) + 12(name) + 2(descLen) + 3(desc)
	const want = 7 + 1 + len("scrcpy-touch") + 2 + 3
	if len(msg) != want {
		t.Fatalf("length = %d, want %d", len(msg), want)
	}
	if msg[0] != scrcpy.TypeUhidCreate {
		t.Fatalf("type = %d", msg[0])
	}
	if msg[1] != 0 || msg[2] != scrcpy.HIDIDTouch {
		t.Fatalf("id = %x %x", msg[1], msg[2])
	}
	if msg[3] != 0x18 || msg[4] != 0xD1 || msg[5] != 0xFF || msg[6] != 0xFF {
		t.Fatalf("vid/pid = %x", msg[3:7])
	}
	if msg[7] != byte(len("scrcpy-touch")) || string(msg[8:20]) != "scrcpy-touch" {
		t.Fatalf("name = %x", msg[7:20])
	}
	if msg[20] != 0 || msg[21] != byte(len(desc)) {
		t.Fatalf("desc len = %x", msg[20:22])
	}
	if !bytes.Equal(msg[22:], desc) {
		t.Fatalf("desc = %x", msg[22:])
	}
}

func TestUhidInputLayout(t *testing.T) {
	report := []byte{1, 2, 0x03, 0xFF, 0x7F}
	msg := scrcpy.UhidInput(scrcpy.HIDIDTouch, report)
	if len(msg) != 5+len(report) || msg[0] != scrcpy.TypeUhidInput {
		t.Fatalf("unexpected msg %x", msg)
	}
	if msg[1] != 0 || msg[2] != scrcpy.HIDIDTouch || msg[3] != 0 || msg[4] != byte(len(report)) {
		t.Fatalf("header = %x", msg[:5])
	}
	if !bytes.Equal(msg[5:], report) {
		t.Fatalf("data = %x", msg[5:])
	}
}

func TestUhidDestroyLayout(t *testing.T) {
	msg := scrcpy.UhidDestroy(scrcpy.HIDIDTouch)
	if len(msg) != 3 || msg[0] != scrcpy.TypeUhidDestroy || msg[1] != 0 || msg[2] != scrcpy.HIDIDTouch {
		t.Fatalf("unexpected msg %x", msg)
	}
}

func TestHidTouchReportLayout(t *testing.T) {
	rep := hidTouchReport(7, 1, true, 1234, 5678, 1023)
	if len(rep) != hidTouchReportLen {
		t.Fatalf("length = %d", len(rep))
	}
	if rep[0] != 1 { // contact count = 1
		t.Fatalf("count = %x", rep[0])
	}
	if rep[1] != 7 {
		t.Fatalf("id = %x", rep[1])
	}
	if rep[2] != 0x03 { // tip switch + in range
		t.Fatalf("flags = %x", rep[2])
	}
	if rep[3] != 1234&0xFF || rep[4] != 1234>>8 || rep[5] != 5678&0xFF || rep[6] != 5678>>8 {
		t.Fatalf("xy = %x", rep[3:7])
	}
	if rep[7] != 0xFF || rep[8] != 0x03 {
		t.Fatalf("pressure = %x", rep[7:9])
	}

	// 抬笔帧：count=0、无 tip/in-range、坐标与压力不再有意义。
	up := hidTouchReport(7, 0, false, 0, 0, 0)
	if up[0] != 0 || up[2] != 0 {
		t.Fatalf("up report = %x", up)
	}
}

func TestHidScale(t *testing.T) {
	if hidScaleX(0, 1080) != 0 || hidScaleY(0, 1920) != 0 {
		t.Fatal("origin must map to origin")
	}
	if hidScaleX(1079, 1080) != 32767 || hidScaleY(1919, 1920) != 32767 {
		t.Fatal("max pixel must map to 32767")
	}
	if hidScaleX(540, 1080) != 16398 {
		t.Fatalf("mid = %d", hidScaleX(540, 1080))
	}
	if hidScaleX(5, 0) != 0 || hidScaleY(5, 1) != 0 {
		t.Fatal("degenerate size must map to 0")
	}
	if hidScalePressure(0.5) != 512 && hidScalePressure(0.5) != 511 {
		t.Fatalf("pressure = %d", hidScalePressure(0.5))
	}
	if hidScalePressure(1.5) != 1023 || hidScalePressure(-1) != 0 {
		t.Fatal("pressure clamp failed")
	}
}
