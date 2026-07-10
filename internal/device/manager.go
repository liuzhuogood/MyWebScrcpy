package device

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Device 表示一台通过 adb 连接的设备。
type Device struct {
	Serial      string `json:"serial"`
	State       string `json:"state"`
	Product     string `json:"product"`
	Model       string `json:"model"`
	DeviceName  string `json:"device"`
	TransportID string `json:"transport_id"`
}

// Manager 负责跟踪 adb 设备列表。
type Manager struct {
	adbPath string
	mu      sync.RWMutex
	devices []Device
	stop    chan struct{}
}

// NewManager 创建设备管理器。adbPath 为 adb 可执行文件路径。
func NewManager(adbPath string) *Manager {
	return &Manager{adbPath: adbPath, stop: make(chan struct{})}
}

// Start 启动后台轮询，定期刷新设备列表。
func (m *Manager) Start() {
	go m.loop()
}

// Stop 停止轮询。
func (m *Manager) Stop() {
	close(m.stop)
}

func (m *Manager) loop() {
	// 首次立即刷新
	m.refresh()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.refresh()
		}
	}
}

// refresh 执行 adb devices -l 并解析输出。
func (m *Manager) refresh() {
	devices, err := m.ListDevices()
	if err != nil {
		return
	}
	m.mu.Lock()
	m.devices = devices
	m.mu.Unlock()
}

// ListDevices 执行一次 adb devices -l 并返回解析结果。
func (m *Manager) ListDevices() ([]Device, error) {
	out, err := exec.Command(m.adbPath, "devices", "-l").Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}
	return parseDevices(string(out)), nil
}

// parseDevices 解析 adb devices -l 的输出。
// 输出格式示例：
//
//	List of devices attached
//	10.0.0.104:5555        device product:PD2072 model:V2072A device:PD2072 transport_id:1
func parseDevices(s string) []Device {
	var devices []Device
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}
		// 第一个 token 是 serial，第二个是 state，其余是 key:value
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{Serial: fields[0], State: fields[1]}
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

// Devices 返回当前缓存的设备列表（只含 online 的 device 状态）。
func (m *Manager) Devices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var online []Device
	for _, d := range m.devices {
		if d.State == "device" {
			// 返回副本
			online = append(online, d)
		}
	}
	return online
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
