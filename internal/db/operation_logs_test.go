package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOperationLogRetentionPaginationAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UnixMilli()
	items := []OperationLog{
		{ID: "one", Serial: "a", Type: "tap", Description: "点击", ADBCommand: "adb shell input tap 1 2", OccurredAt: now},
		{ID: "two", Serial: "b", Type: "key", Description: "返回", ADBCommand: "adb shell input keyevent 4", OccurredAt: now},
		{ID: "three", Serial: "a", Type: "text", Description: "输入", ADBCommand: "adb shell input text hello", OccurredAt: now},
	}
	if err = d.AppendOperationLogs(ctx, items); err != nil {
		t.Fatal(err)
	}
	if err = d.AppendOperationLogs(ctx, items); err != nil {
		t.Fatal(err)
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	page, cursor, err := d.ListOperationLogs(ctx, "", 0, 2)
	if err != nil || len(page) != 2 || page[0].ID != "three" || page[1].ID != "two" || cursor == 0 {
		t.Fatalf("page=%+v cursor=%d err=%v", page, cursor, err)
	}
	page, cursor, err = d.ListOperationLogs(ctx, "", cursor, 2)
	if err != nil || len(page) != 1 || page[0].ID != "one" || cursor != 0 {
		t.Fatalf("page=%+v cursor=%d err=%v", page, cursor, err)
	}
	page, _, err = d.ListOperationLogs(ctx, "a", 0, 100)
	if err != nil || len(page) != 2 {
		t.Fatalf("device page=%+v err=%v", page, err)
	}
	// Seed an expired row to prove reads hide it even before periodic pruning.
	expired := now - OperationLogRetention.Milliseconds() - 1
	if _, err = d.db.Exec(`INSERT INTO operation_logs(id,serial,type,description,adb_command,occurred_at) VALUES('old','a','tap','old','old',?)`, expired); err != nil {
		t.Fatal(err)
	}
	if err = d.AppendOperationLogs(ctx, []OperationLog{{ID: "late", Serial: "a", OccurredAt: expired}}); err != nil {
		t.Fatal(err)
	}
	page, _, err = d.ListOperationLogs(ctx, "", 0, 100)
	if err != nil || len(page) != 3 {
		t.Fatalf("expired rows leaked: %+v, %v", page, err)
	}
	if err = d.PruneOperationLogs(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = d.db.QueryRow(`SELECT COUNT(*) FROM operation_logs`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("physical count=%d err=%v", count, err)
	}
	deleted, err := d.ClearOperationLogs(ctx, "a")
	if err != nil || deleted != 2 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	page, _, err = d.ListOperationLogs(ctx, "", 0, 100)
	if err != nil || len(page) != 1 || page[0].Serial != "b" {
		t.Fatalf("other device lost: %+v %v", page, err)
	}
	deleted, err = d.ClearOperationLogs(ctx, "")
	if err != nil || deleted != 1 {
		t.Fatalf("delete all=%d err=%v", deleted, err)
	}
}
