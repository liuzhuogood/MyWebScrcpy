package taskartifact

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFragmentsDrainAndCursor(t *testing.T) {
	s, e := NewLogStore(filepath.Join(t.TempDir(), "logs"), 40000, 100000)
	if e != nil {
		t.Fatal(e)
	}
	w, e := s.Create("task_a", func(LogState) {})
	if e != nil {
		t.Fatal(e)
	}
	out := w.Writer("stdout")
	out.Write([]byte("中文\n"))
	out.Write(bytes.Repeat([]byte("界"), 20000))
	out.Write([]byte("\nlast\n"))
	w.Close()
	page, e := s.Read("task_a", LogQuery{Limit: 500})
	if e != nil || len(page.Items) < 2 || !page.Truncated {
		t.Fatalf("%+v %v", page, e)
	}
	for _, line := range page.Items {
		if strings.Contains(line.Message, "�") {
			t.Fatal("UTF-8 split")
		}
	}
	page, e = s.Read("task_a", LogQuery{After: 1, HasAfter: true, Limit: 1, Stream: "stderr"})
	if e != nil || page.NextCursor <= 1 || len(page.Items) != 0 {
		t.Fatalf("filtered cursor stuck %+v %v", page, e)
	}
}
func TestUTF8AcrossPipeReadsAndLiveQueries(t *testing.T) {
	s, _ := NewLogStore(t.TempDir(), 1<<20, 2<<20)
	w, _ := s.Create("utf8", nil)
	out := w.Writer("stdout")
	word := []byte("中文\n")
	for _, b := range word {
		out.Write([]byte{b})
	}
	w.Close()
	page, e := s.Read("utf8", LogQuery{Limit: 500})
	if e != nil {
		t.Fatal(e)
	}
	var text string
	for _, line := range page.Items {
		text += line.Message
	}
	if text != "中文" {
		t.Fatalf("UTF-8 corrupted: %q", text)
	}
	w, _ = s.Create("live", nil)
	out = w.Writer("stdout")
	done := make(chan struct{})
	go func() {
		for i := 0; i < 300; i++ {
			out.Write([]byte("输出\n"))
		}
		w.Close()
		close(done)
	}()
	for {
		if _, e := s.Read("live", LogQuery{Limit: 20}); e != nil {
			t.Fatal(e)
		}
		select {
		case <-done:
			return
		default:
		}
	}
}
func TestDegradedAndExpiredLogs(t *testing.T) {
	s, _ := NewLogStore(t.TempDir(), 1<<20, 2<<20)
	w, _ := s.Create("broken", nil)
	w.file.Close()
	if _, e := w.Writer("stdout").Write([]byte("still drains")); e != nil {
		t.Fatal(e)
	}
	w.Close()
	st, e := s.Status("broken")
	if e != nil || st.CaptureState != "degraded" {
		t.Fatalf("%+v %v", st, e)
	}
}
func TestLogQuotaRetainedStatusAndBidirectionalPagination(t *testing.T) {
	s, _ := NewLogStore(t.TempDir(), 30000, 40000)
	w, _ := s.Create("pages", nil)
	out := w.Writer("stdout")
	for i := 0; i < 400; i++ {
		out.Write([]byte("entry\n"))
	}
	w.Close()
	first, e := s.Read("pages", LogQuery{Limit: 10})
	if e != nil || len(first.Items) != 10 || !first.HasMore {
		t.Fatalf("%+v %v", first, e)
	}
	next, e := s.Read("pages", LogQuery{Before: first.NextCursor, HasBefore: true, Limit: 10})
	if e != nil || len(next.Items) != 10 || next.Items[9].Seq >= first.Items[0].Seq {
		t.Fatalf("repeated rows %+v %v", next, e)
	}
	if p, e := s.Read("pages", LogQuery{After: 1<<63 - 1, HasAfter: true}); e != nil || len(p.Items) != 0 {
		t.Fatalf("overflow cursor %+v %v", p, e)
	}
	path, _ := s.path("pages", ".state.json")
	st, _ := s.Status("pages")
	expired := int64(1)
	st.ExpiresAt = &expired
	b, _ := json.Marshal(st)
	os.WriteFile(path, b, 0600)
	s.Cleanup()
	page, e := s.Read("pages", LogQuery{})
	if e != nil || page.Availability != "expired" {
		t.Fatalf("%+v %v", page, e)
	}
	if _, e = s.Open("pages"); e == nil {
		t.Fatal("expired download available")
	}
	s.mu.Lock()
	total := s.total
	s.mu.Unlock()
	if total != 0 {
		t.Fatalf("quota not freed %d", total)
	}
}

func TestIndexFailureRollsBackQuotaAndTruncationStopsSaving(t *testing.T) {
	s, _ := NewLogStore(t.TempDir(), 160, 1000)
	w, _ := s.Create("broken_index", nil)
	w.index.Close()
	w.Writer("stdout").Write([]byte("first"))
	w.Close()
	if s.total != 0 {
		t.Fatalf("quota retained after index failure: %d", s.total)
	}
	w, _ = s.Create("capped", nil)
	out := w.Writer("stdout")
	out.Write(bytes.Repeat([]byte("a"), 1000))
	out.Write([]byte("small"))
	w.Close()
	state, _ := s.Status("capped")
	if !state.Truncated || state.LastSeq != 0 {
		t.Fatalf("saved after truncation %+v", state)
	}
}
