package device

import (
	"context"
	"mywebscrcpy/internal/dependencies"
	"os"
	"strconv"
	"strings"
	"time"
)

// HardwareInfo 表示设备的静态硬件与系统属性（生命周期内基本不变，仅需拉取一次缓存）。
type HardwareInfo struct {
	Brand          string
	Manufacturer   string
	Model          string
	AndroidRelease string
	SDKVersion     int
	CPUABI         string
	BuildDisplayID string
}

// TelemetryInfo 表示设备的动态运行遥测指标（电量、双温感等，由后台独立协程异步轮询更新）。
type TelemetryInfo struct {
	BatteryLevel     *int
	BatteryTemp      *float64
	BatteryCharging  *bool
	CPUTemp          *float64
	CPUTempSupported bool
	UpdatedAt        int64
}

// parseHardwareProps 解析多段 getprop 输出
func parseHardwareProps(output string) HardwareInfo {
	parts := strings.Split(output, "===")
	info := HardwareInfo{}
	clean := func(s string) string {
		return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
	}

	if len(parts) > 0 {
		info.Brand = clean(parts[0])
	}
	if len(parts) > 1 {
		info.Manufacturer = clean(parts[1])
	}
	if len(parts) > 2 {
		info.Model = clean(parts[2])
	}
	if len(parts) > 3 {
		info.AndroidRelease = clean(parts[3])
	}
	if len(parts) > 4 {
		if sdk, err := strconv.Atoi(clean(parts[4])); err == nil {
			info.SDKVersion = sdk
		}
	}
	if len(parts) > 5 {
		info.CPUABI = clean(parts[5])
	}
	if len(parts) > 6 {
		info.BuildDisplayID = clean(parts[6])
	}
	return info
}

// parseBatteryAndThermal 解析 dumpsys battery 及 thermal_zone 输出
func parseBatteryAndThermal(output string) TelemetryInfo {
	info := TelemetryInfo{
		UpdatedAt: time.Now().UnixMilli(),
	}

	sections := strings.Split(output, "===THERMAL===")
	batteryPart := sections[0]

	// 解析 dumpsys battery
	lines := strings.Split(batteryPart, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "level:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if lvl, err := strconv.Atoi(parts[1]); err == nil {
					info.BatteryLevel = &lvl
				}
			}
		} else if strings.HasPrefix(line, "temperature:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if rawTemp, err := strconv.ParseFloat(parts[1], 64); err == nil {
					// Android dumpsys battery temperature 通常以 0.1 摄氏度为单位 (例如 365 -> 36.5°C)
					temp := rawTemp
					if temp > 100 {
						temp = temp / 10.0
					}
					info.BatteryTemp = &temp
				}
			}
		} else if strings.HasPrefix(line, "status:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Android 电池状态: 2 = BATTERY_STATUS_CHARGING, 5 = BATTERY_STATUS_FULL
				isCharging := parts[1] == "2" || parts[1] == "5"
				info.BatteryCharging = &isCharging
			}
		}
	}

	// 解析 thermal_zone
	if len(sections) > 1 {
		thermalLines := strings.Split(sections[1], "\n")
		var validTemps []float64
		for _, rawLine := range thermalLines {
			line := strings.TrimSpace(rawLine)
			if line == "" || !strings.Contains(line, ":") {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				valStr := strings.TrimSpace(parts[1])
				if val, err := strconv.ParseFloat(valStr, 64); err == nil {
					// 常见单位: 42000 表示 42.0°C; 若原本即为 42.0 则直接使用
					temp := val
					if temp > 1000 {
						temp = temp / 1000.0
					}
					// 过滤合理 CPU 硬件温度区间 (20°C ~ 110°C)
					if temp >= 20.0 && temp <= 110.0 {
						validTemps = append(validTemps, temp)
					}
				}
			}
		}

		if len(validTemps) > 0 {
			// 取检测到的第一个有效 CPU/热区温度
			cpuTemp := validTemps[0]
			info.CPUTemp = &cpuTemp
			info.CPUTempSupported = true
		}
	}

	return info
}

// runHardwareFetch 异步执行 adb 命令获取设备静态硬件属性
func (m *Manager) runHardwareFetch(serial string) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	cmdStr := "getprop ro.product.brand; echo '==='; getprop ro.product.manufacturer; echo '==='; getprop ro.product.model; echo '==='; getprop ro.build.version.release; echo '==='; getprop ro.build.version.sdk; echo '==='; getprop ro.product.cpu.abi; echo '==='; getprop ro.build.display.id"
	cmd := dependencies.CommandContext(ctx, m.adbPath, "-s", serial, "shell", cmdStr)
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	if err != nil {
		m.telemetryMu.Lock()
		delete(m.fetchingHW, serial)
		m.telemetryMu.Unlock()
		return
	}

	hw := parseHardwareProps(string(out))
	m.telemetryMu.Lock()
	m.hardwareCache[serial] = hw
	delete(m.fetchingHW, serial)
	m.telemetryMu.Unlock()
}

// runTelemetryFetch 异步执行 adb 命令获取单台设备的电量与温度
func (m *Manager) runTelemetryFetch(serial string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdStr := "dumpsys battery; echo '===THERMAL==='; for f in /sys/class/thermal/thermal_zone*/temp; do [ -f \"$f\" ] && echo \"$f:$(cat \"$f\" 2>/dev/null)\"; done"
	cmd := dependencies.CommandContext(ctx, m.adbPath, "-s", serial, "shell", cmdStr)
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	if err != nil {
		return
	}

	tel := parseBatteryAndThermal(string(out))
	m.telemetryMu.Lock()
	m.telemetryCache[serial] = tel
	m.telemetryMu.Unlock()
}

// telemetryLoop 后台独立守护协程，按 6 秒间隔静默轮询在线设备，零阻塞主接口
func (m *Manager) telemetryLoop() {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.pollOnlineDevicesTelemetry()
		}
	}
}

// pollOnlineDevicesTelemetry 遍历在线设备并串行采集遥测（避免打满 adb 连接）
func (m *Manager) pollOnlineDevicesTelemetry() {
	m.mu.RLock()
	var onlineSerials []string
	for _, d := range m.devices {
		if d.State == "device" {
			onlineSerials = append(onlineSerials, d.Serial)
		}
	}
	m.mu.RUnlock()

	for _, serial := range onlineSerials {
		select {
		case <-m.stop:
			return
		default:
			m.runTelemetryFetch(serial)
		}
	}
}
