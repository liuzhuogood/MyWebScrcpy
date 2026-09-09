package ws

import "testing"

func TestModifiersToMeta(t *testing.T) {
	meta, err := modifiersToMeta([]string{"ctrl", "shift", "meta"})
	if err != nil || meta != metaCtrl|metaShift|metaMeta {
		t.Fatalf("meta=%x err=%v", meta, err)
	}
	if _, err := modifiersToMeta([]string{"ctrl", "ctrl"}); err == nil {
		t.Fatal("duplicate modifier accepted")
	}
	if _, err := modifiersToMeta([]string{"super"}); err == nil {
		t.Fatal("unknown modifier accepted")
	}
}
