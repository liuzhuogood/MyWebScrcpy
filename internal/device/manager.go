package device

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
)

// Device 表示一台设备，融合了 adb 动态状态与持久化配置。
type Device struct {
	Serial        string `json:"serial"`
	State         string `json:"state"` // "device", "offline", "connecting", "unauthorized" 等
	Source        string `json:"source"` // "auto" (自动发现) 或 "manual" (手动添加)
	Alias         string `json:"alias,omitempty"`
	Product       string `json:"product,omitempty"`
	Model         string `json:"model,omitempty"`
	DeviceName    string `json:"device,omitempty"`
	TransportID   string `json:"transport_id,omitempty"`
	AutoReconnect bool   `json:"auto_reconnect"`
}

// Manager 负责跟踪 adb 设备列表并守护手动添加设备的连接与持久化。
type Manager struct {
	adbPath  string
	db       *db.DB
	mu       sync.RWMutex
	devices  []Device
	stop     chan struct{}
}

// NewManager 创建设备管理器。
func NewManager(adbPath string, database *db.DB) *Manager {
	return &Manager{
		adbPath: adbPath,
		db:      database,
		stop:    make(chan struct{}),
	}
}

// Start 启动后台轮询，定期刷新设备列表和自动重连守护。
func (m *Manager) Start() {
	go m.loop()
}

// Stop 停止轮询。
func (m *Manager) Stop() {
	close(m.stop)
}

func (m *Manager) loop() {
	m.refresh()
	ticker := time.NewTicker(2 * time.Second)
	reconnectTicker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	defer reconnectTicker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.refresh()
		case <-reconnectTicker.C:
			m.reconnectOfflineManualDevices()
		}
	}
}

// refresh 执行 adb devices -l 并与 SQLite 手动设备合并。
func (m *Manager) refresh() {
	adbDevs, err := m.ListDevices()
	if err != nil {
		adbDevs = nil
	}

	adbMap := make(map[string]Device)
	for _, d := range adbDevs {
		adbMap[d.Serial] = d
	}

	var manualList []db.DeviceRecord
	if m.db != nil {
		manualList, _ = m.db.ListManualDevices()
	}

	manualMap := make(map[string]db.DeviceRecord)
	for _, rec := range manualList {
		manualMap[rec.Serial] = rec
	}

	var combined []Device
	seen := make(map[string]bool)

	// 1. 处理数据库中的手动设备
	for _, rec := range manualList {
		seen[rec.Serial] = true
		d := Device{
			Serial:        rec.Serial,
			Source:        "manual",
			Alias:         rec.Alias,
			State:         "offline",
			Product:       rec.Product,
			Model:         rec.Model,
			DeviceName:    rec.DeviceName,
			TransportID:   rec.TransportID,
			AutoReconnect: rec.AutoReconnect == 1,
		}

		if adbDev, ok := adbMap[rec.Serial]; ok {
			d.State = adbDev.State
			if adbDev.Product != "" {
				d.Product = adbDev.Product
			}
			if adbDev.Model != "" {
				d.Model = adbDev.Model
			}
			if adbDev.DeviceName != "" {
				d.DeviceName = adbDev.DeviceName
			}
			if adbDev.TransportID != "" {
				d.TransportID = adbDev.TransportID
			}

			// 异步回写 DB 最新状态
			if m.db != nil && (rec.State != d.State || rec.Model != d.Model) {
				go m.db.UpdateDeviceStatus(d.Serial, d.State, d.Product, d.Model, d.DeviceName, d.TransportID, d.State == "device")
			}
		} else {
			if m.db != nil && rec.State != "offline" && rec.State != "connecting" {
				go m.db.UpdateDeviceStatus(d.Serial, "offline", "", "", "", "", false)
			}
		}
		combined = append(combined, d)
	}

	// 2. 处理 ADB 自动发现的设备（未在手动列表中的）
	for _, adbDev := range adbDevs {
		if !seen[adbDev.Serial] {
			adbDev.Source = "auto"
			combined = append(combined, adbDev)
		}
	}

	m.mu.Lock()
	m.devices = combined
	m.mu.Unlock()
}

// reconnectOfflineManualDevices 扫描开启了 auto_reconnect 且离线的手动设备，执行 adb connect
func (m *Manager) reconnectOfflineManualDevices() {
	if m.db == nil {
		return
	}
	manuals, err := m.db.ListManualDevices()
	if err != nil {
		return
	}

	for _, rec := range manuals {
		if rec.AutoReconnect == 1 && rec.State == "offline" {
			go m.Connect(rec.Serial)
		}
	}
}

// Connect 调用 adb connect <serial>
func (m *Manager) Connect(serial string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.adbPath, "connect", serial)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		return fmt.Errorf("adb connect 错误: %s (%w)", strings.TrimSpace(output), err)
	}

	// 触发一次即刻刷新（包含 failed to authenticate 变为 unauthorized 的情况）
	m.refresh()

	// adb connect 成功通常输出: connected to ... 或 already connected to ...
	// 未授权时输出: failed to authenticate to ...（此时已建立连接并处于 unauthorized 状态）
	if (strings.Contains(output, "failed") && !strings.Contains(output, "failed to authenticate")) || strings.Contains(output, "cannot connect") {
		return fmt.Errorf("adb connect 失败: %s", strings.TrimSpace(output))
	}

	return nil
}

// AddManualDevice 添加手动设备并尝试首次连接
func (m *Manager) AddManualDevice(serial, alias string) (*Device, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil, fmt.Errorf("设备地址不能为空")
	}

	if m.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	rec, err := m.db.UpsertManualDevice(serial, alias, 1)
	if err != nil {
		return nil, fmt.Errorf("保存手动设备失败: %w", err)
	}

	// 异步尝试 ADB 连接，不阻塞响应
	go func() {
		_ = m.Connect(serial)
	}()

	dev := &Device{
		Serial:        rec.Serial,
		Source:        "manual",
		Alias:         rec.Alias,
		State:         "connecting",
		AutoReconnect: true,
	}

	m.refresh()
	return dev, nil
}

// RemoveManualDevice 移除手动设备
func (m *Manager) RemoveManualDevice(serial string) error {
	if m.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	if err := m.db.DeleteManualDevice(serial); err != nil {
		return err
	}

	// 可选尝试断开 adb disconnect
	go func() {
		cmd := exec.Command(m.adbPath, "disconnect", serial)
		cmd.Dir = os.TempDir()
		_ = cmd.Run()
	}()

	m.refresh()
	return nil
}

// UpdateAlias 更新别名
func (m *Manager) UpdateAlias(serial, alias string) error {
	if m.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	if err := m.db.UpdateAlias(serial, alias); err != nil {
		return err
	}

	m.refresh()
	return nil
}

// ListDevices 执行一次 adb devices -l 并返回解析结果。
func (m *Manager) ListDevices() ([]Device, error) {
	cmd := exec.Command(m.adbPath, "devices", "-l")
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}
	return parseDevices(string(out)), nil
}

// parseDevices 解析 adb devices -l 的输出。
func parseDevices(s string) []Device {
	var devices []Device
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{
			Serial: fields[0],
			State:  fields[1],
			Source: "auto",
		}
		for _, f := range fields[2:] {
			kv := strings.SplitN(f, ":", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "product":
				d.Product = kv[1]
			case "model":
				d.Model = kv[1]
			case "device":
				d.DeviceName = kv[1]
			case "transport_id":
				d.TransportID = kv[1]
			}
		}
		devices = append(devices, d)
	}
	return devices
}

// Devices 返回当前缓存的全部设备列表（在线或配置的手动设备）。
func (m *Manager) Devices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]Device, len(m.devices))
	copy(res, m.devices)
	return res
}

// GetDevice 返回指定 serial 的设备。
func (m *Manager) GetDevice(serial string) (Device, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.devices {
		if d.Serial == serial && d.State == "device" {
			return d, true
		}
	}
	return Device{}, false
}
