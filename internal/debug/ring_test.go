package debug

import (
	"testing"
	"time"
)

func TestRingKeepsNewestEventsAndFiltersDevice(t *testing.T) {
	r := New(2)
	r.Append(Event{At: time.Unix(1, 0), Type: "one", DeviceID: "a"})
	r.Append(Event{At: time.Unix(2, 0), Type: "two", DeviceID: "b"})
	r.Append(Event{At: time.Unix(3, 0), Type: "three", DeviceID: "a"})
	events := r.Snapshot("a", 10)
	if len(events) != 1 || events[0].Type != "three" {
		t.Fatalf("unexpected filtered events: %+v", events)
	}
	all := r.Snapshot("", 10)
	if len(all) != 2 || all[0].Type != "three" || all[1].Type != "two" {
		t.Fatalf("unexpected ring order: %+v", all)
	}
}

func TestRingCopiesFields(t *testing.T) {
	r := New(2)
	fields := map[string]interface{}{"count": 1}
	r.Append(Event{Type: "copy", Fields: fields})
	fields["count"] = 2
	got := r.Snapshot("", 1)
	if got[0].Fields["count"] != 1 {
		t.Fatalf("event fields were not copied: %+v", got[0].Fields)
	}
}
