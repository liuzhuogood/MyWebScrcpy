package db

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestTaskRunCreateAndDeduplication(t *testing.T) {
	d := newTestDB(t)

	run, created, err := d.CreateTaskRun(&TaskRunRecord{
		ClientRunID:  "client-1",
		Name:         "测试任务",
		DeviceSerial: "serial-1",
		Source:       "codex",
	})
	if err != nil {
		t.Fatalf("CreateTaskRun failed: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true")
	}
	if run.ID == "" || run.Status != "running" {
		t.Fatalf("unexpected run: %+v", run)
	}

	// 相同 client_run_id 与相同内容幂等返回
	same, created2, err := d.CreateTaskRun(&TaskRunRecord{
		ClientRunID:  "client-1",
		Name:         "测试任务",
		DeviceSerial: "serial-1",
		Source:       "codex",
	})
	if err != nil {
		t.Fatalf("idempotent create failed: %v", err)
	}
	if created2 {
		t.Fatalf("expected created=false for duplicate")
	}
	if same.ID != run.ID {
		t.Fatalf("id mismatch: %s != %s", same.ID, run.ID)
	}

	// 相同 client_run_id 不同内容返回 ErrConflict
	_, _, err = d.CreateTaskRun(&TaskRunRecord{
		ClientRunID:  "client-1",
		Name:         "冲突任务名",
		DeviceSerial: "serial-1",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got: %v", err)
	}
}

func TestTaskRunHeartbeatAndFinality(t *testing.T) {
	d := newTestDB(t)

	run, _, err := d.CreateTaskRun(&TaskRunRecord{
		ClientRunID:  "client-hb",
		Name:         "心跳任务",
		DeviceSerial: "serial-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UnixMilli() + 1000
	updated, err := d.HeartbeatTaskRun(run.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastHeartbeatAt != now {
		t.Fatalf("heartbeat timestamp not updated: %d != %d", updated.LastHeartbeatAt, now)
	}

	// 提交终态
	exitCode := 0
	finished, err := d.FinishTaskRun(run.ID, "succeeded", &exitCode, "运行成功", "# 报告内容", now+500)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "succeeded" {
		t.Fatalf("status mismatch: %s", finished.Status)
	}

	// 终态后迟到心跳不可回退终态
	hbAfterFinish, err := d.HeartbeatTaskRun(run.ID, now+2000)
	if err != nil {
		t.Fatal(err)
	}
	if hbAfterFinish.Status != "succeeded" {
		t.Fatalf("final status changed after heartbeat: %s", hbAfterFinish.Status)
	}
}

func TestTaskRunLogsDeduplicationAndConflict(t *testing.T) {
	d := newTestDB(t)

	run, _, err := d.CreateTaskRun(&TaskRunRecord{
		ClientRunID:  "client-logs",
		Name:         "日志任务",
		DeviceSerial: "serial-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	entries := []TaskRunLogEntry{
		{Seq: 1, Stream: "stdout", Message: "line 1"},
		{Seq: 2, Stream: "stdout", Message: "line 2"},
	}
	ackSeq, truncated, err := d.AppendTaskRunLogs(run.ID, entries, 100)
	if err != nil {
		t.Fatal(err)
	}
	if ackSeq != 2 || truncated {
		t.Fatalf("unexpected ack=%d, truncated=%v", ackSeq, truncated)
	}

	// 重复上报相同内容幂等忽略
	ackSeq2, _, err := d.AppendTaskRunLogs(run.ID, entries, 100)
	if err != nil {
		t.Fatal(err)
	}
	if ackSeq2 != 2 {
		t.Fatalf("unexpected ack=%d after duplicate append", ackSeq2)
	}

	// 上报相同 seq 但不同内容报 ErrConflict
	conflictEntries := []TaskRunLogEntry{
		{Seq: 2, Stream: "stdout", Message: "different content"},
	}
	_, _, err = d.AppendTaskRunLogs(run.ID, conflictEntries, 100)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got: %v", err)
	}

	// 容量截断测试
	entriesMore := []TaskRunLogEntry{
		{Seq: 3, Stream: "stdout", Message: "line 3"},
		{Seq: 4, Stream: "stdout", Message: "line 4"},
	}
	ackSeq3, truncated3, err := d.AppendTaskRunLogs(run.ID, entriesMore, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated3 {
		t.Fatalf("expected truncated=true when exceeding maxLogs=3")
	}
	if ackSeq3 != 3 {
		t.Fatalf("expected ack=3, got %d", ackSeq3)
	}

	// 验证日志查询
	logs, lastSeq, wasTrunc, err := d.GetTaskRunLogs(run.ID, 0, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 || !wasTrunc || lastSeq != 3 {
		t.Fatalf("unexpected logs: len=%d, lastSeq=%d, trunc=%v", len(logs), lastSeq, wasTrunc)
	}
}

func TestTaskRunFinishConflict(t *testing.T) {
	d := newTestDB(t)

	run, _, err := d.CreateTaskRun(&TaskRunRecord{
		ClientRunID:  "client-finish",
		Name:         "结束冲突任务",
		DeviceSerial: "serial-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	exit0 := 0
	_, err = d.FinishTaskRun(run.ID, "succeeded", &exit0, "OK", "", 1000)
	if err != nil {
		t.Fatal(err)
	}

	// 相同 finish 幂等
	same, err := d.FinishTaskRun(run.ID, "succeeded", &exit0, "OK", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if same.Status != "succeeded" {
		t.Fatalf("expected succeeded")
	}

	// 冲突终态拒绝
	exit1 := 1
	_, err = d.FinishTaskRun(run.ID, "failed", &exit1, "Failed", "", 2000)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for finish conflict, got: %v", err)
	}
}

func TestReviewChangedFinishPayloadConflicts(t *testing.T) {
	d := newTestDB(t)
	r, _, err := d.CreateTaskRun(&TaskRunRecord{ClientRunID: "rev-finish", Name: "review", DeviceSerial: "test-serial", Source: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	code := 0
	if _, err := d.FinishTaskRun(r.ID, "succeeded", &code, "original", "original", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishTaskRun(r.ID, "succeeded", &code, "different", "different", time.Now().UnixMilli()); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed final result must conflict, got %v", err)
	}
}

func TestReviewChangedCreateSourceConflicts(t *testing.T) {
	d := newTestDB(t)
	if _, _, err := d.CreateTaskRun(&TaskRunRecord{ClientRunID: "rev-source", Name: "review", DeviceSerial: "test-serial", Source: "codex"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.CreateTaskRun(&TaskRunRecord{ClientRunID: "rev-source", Name: "review", DeviceSerial: "test-serial", Source: "other"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed creation metadata must conflict, got %v", err)
	}
}

func TestReviewAcknowledgementDoesNotSkipMissingLogs(t *testing.T) {
	d := newTestDB(t)
	r, _, err := d.CreateTaskRun(&TaskRunRecord{ClientRunID: "rev-ack", Name: "review", DeviceSerial: "test-serial", Source: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	ack, _, err := d.AppendTaskRunLogs(r.ID, []TaskRunLogEntry{{Seq: 2, Stream: "stdout", Message: "second only"}}, 100)
	if err == nil && ack >= 2 {
		t.Fatalf("acknowledged log 2 while log 1 is missing: ack=%d", ack)
	}
}

func TestReviewFinishedRunRejectsNewLog(t *testing.T) {
	d := newTestDB(t)
	r, _, err := d.CreateTaskRun(&TaskRunRecord{ClientRunID: "rev-freeze", Name: "review", DeviceSerial: "test-serial", Source: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	code := 0
	if _, err := d.FinishTaskRun(r.ID, "succeeded", &code, "finished", "", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.AppendTaskRunLogs(r.ID, []TaskRunLogEntry{{Seq: 1, Stream: "stdout", Message: "after final"}}, 100); err == nil {
		t.Fatal("finished run accepted a new log after evidence was frozen")
	}
}
