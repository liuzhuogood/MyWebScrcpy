package db

import (
	"path/filepath"
	"testing"
)

func TestDispatchSummaryUsesExplicitBatchAndSurvivesTargetChange(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "dispatch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	script, err := d.CreateMarkdownScript("汇总", []string{"device-A"}, "version: 1.0.0", "1.0.0", "", 0, 120)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := d.BeginScriptDispatch(script.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.InsertRunningTask("first", script.ID, script.Name, "device-A", 100); err != nil {
		t.Fatal(err)
	}
	if err := d.AttachDispatchTask(first, "first"); err != nil {
		t.Fatal(err)
	}
	second, _, err := d.BeginScriptDispatch(script.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.InsertRunningTask("second", script.ID, script.Name, "device-B", 101); err != nil {
		t.Fatal(err)
	}
	if err := d.AttachDispatchTask(second, "second"); err != nil {
		t.Fatal(err)
	}
	// Deliberately make task timestamps identical; they cannot identify a dispatch.
	if _, err := d.db.Exec(`UPDATE running_tasks SET started_at=500`); err != nil {
		t.Fatal(err)
	}
	if err := d.FinishRunningTask("first", "failed", 1, "旧批次失败"); err != nil {
		t.Fatal(err)
	}
	if err := d.ReconcileScriptRunStatus(script.ID); err != nil {
		t.Fatal(err)
	}
	saved, _ := d.GetMarkdownScript(script.ID)
	if saved.LastRunStatus != "started" {
		t.Fatalf("old dispatch overwrote latest: %s", saved.LastRunStatus)
	}
	if err := d.UpdateMarkdownScript(script.ID, script.Name, []string{"device-C"}, script.Content, "1.0.0", "", 0, 120); err != nil {
		t.Fatal(err)
	}
	if err := d.FinishRunningTask("second", "completed", 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.ReconcileScriptRunStatus(script.ID); err != nil {
		t.Fatal(err)
	}
	saved, _ = d.GetMarkdownScript(script.ID)
	if saved.LastRunStatus != "completed" || saved.TargetDevices[0] != "device-C" {
		t.Fatalf("summary/configuration wrong: %+v", saved)
	}
}
