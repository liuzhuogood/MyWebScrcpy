package taskartifact

import (
	"encoding/json"
	"mywebscrcpy/internal/db"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrozenReportEvidenceFitsBudgetWithLargeErrors(t *testing.T) {
	d, e := db.Open(filepath.Join(t.TempDir(), "budget.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	s := NewService(d)
	defer s.Close()
	if e = d.InsertRunningTask("budget", 1, "budget", "phone", 0); e != nil {
		t.Fatal(e)
	}
	w, e := s.Create("budget", "", Options{Report: ReportOptions{Enabled: true}})
	if e != nil {
		t.Fatal(e)
	}
	out := w.Writer("stderr")
	for i := 0; i < 80; i++ {
		out.Write([]byte(strings.Repeat("界", 4000) + "\n"))
	}
	w.Close()
	if e = d.FinishRunningTask("budget", "failed", 1, strings.Repeat("\x01界", 30000)); e != nil {
		t.Fatal(e)
	}
	if e = s.Enqueue("budget", false); e != nil {
		t.Fatal(e)
	}
	report, e := d.Report("budget", 0, false)
	if e != nil || report == nil || len(report.Evidence) > 64<<10 {
		t.Fatalf("report=%+v err=%v", report, e)
	}
	var evidence Evidence
	if e = json.Unmarshal([]byte(report.Evidence), &evidence); e != nil {
		t.Fatal(e)
	}
	if !evidence.MetadataTruncated || !evidence.SelectionTruncated {
		t.Fatalf("missing truncation %+v", evidence)
	}
}
