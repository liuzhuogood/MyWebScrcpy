package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDBDeviceLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "testdb-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// 1. 插入手动设备
	rec, err := database.UpsertManualDevice("192.168.1.100:5555", "测试电视盒子", 1)
	if err != nil {
		t.Fatalf("UpsertManualDevice failed: %v", err)
	}
	if rec.Serial != "192.168.1.100:5555" || rec.Source != "manual" || rec.Alias != "测试电视盒子" {
		t.Errorf("Unexpected record: %+v", rec)
	}

	// 2. 列表查询
	list, err := database.ListManualDevices()
	if err != nil {
		t.Fatalf("ListManualDevices failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(list))
	}

	// 3. 更新状态（在线并有产品信息）
	err = database.UpdateDeviceStatus("192.168.1.100:5555", "device", "MiBOX4", "MDZ-22-AB", "aquaman", "1", true)
	if err != nil {
		t.Fatalf("UpdateDeviceStatus failed: %v", err)
	}

	list, _ = database.ListManualDevices()
	if list[0].State != "device" || list[0].Model != "MDZ-22-AB" || list[0].LastConnectedAt == 0 {
		t.Errorf("Device status not updated properly: %+v", list[0])
	}

	// 4. 更新别名
	err = database.UpdateAlias("192.168.1.100:5555", "新客厅电视")
	if err != nil {
		t.Fatalf("UpdateAlias failed: %v", err)
	}
	list, _ = database.ListManualDevices()
	if list[0].Alias != "新客厅电视" {
		t.Errorf("Alias update failed: got %s", list[0].Alias)
	}

	// 5. 删除手动设备
	err = database.DeleteManualDevice("192.168.1.100:5555")
	if err != nil {
		t.Fatalf("DeleteManualDevice failed: %v", err)
	}

	list, _ = database.ListManualDevices()
	if len(list) != 0 {
		t.Errorf("Expected 0 devices after deletion, got %d", len(list))
	}

	// 删除不存在或非手动设备应报错
	err = database.DeleteManualDevice("not_exist")
	if err == nil {
		t.Error("Expected error deleting non-existent device")
	}
}
