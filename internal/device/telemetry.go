package device

import (
	"context"
	"mywebscrcpy/internal/dependencies"
	"os"
	"regexp"
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

// TelemetryInfo 表示设备的动态运行遥测指标（电量、双温感、前台应用等，由后台独立协程异步轮询更新）。
type TelemetryInfo struct {
	BatteryLevel      *int
	BatteryTemp       *float64
	BatteryCharging   *bool
	CPUTemp           *float64
	CPUTempSupported  bool
	ForegroundPkg     string
	ForegroundAppName string
	ForegroundAppIcon string
	UpdatedAt         int64
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

// parseCPUTempFromThermalService 从 dumpsys thermalservice 输出中提取 CPU 核心温度
func parseCPUTempFromThermalService(content string) *float64 {
	lines := strings.Split(content, "\n")
	inHAL := false
	var halCPUTemp *float64
	var cachedCPUTemp *float64

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.Contains(line, "Current temperatures from HAL:") {
			inHAL = true
			continue
		}
		if inHAL && (strings.Contains(line, "Current cooling devices") || strings.Contains(line, "Temperature static thresholds")) {
			inHAL = false
		}

		if !strings.Contains(line, "Temperature{") {
			continue
		}

		start := strings.Index(line, "Temperature{")
		end := strings.Index(line[start:], "}")
		if end == -1 {
			continue
		}
		body := line[start+len("Temperature{") : start+end]
		fields := strings.Split(body, ",")

		var mValue float64
		var mType string
		var mName string
		hasValue := false

		for _, field := range fields {
			kv := strings.SplitN(strings.TrimSpace(field), "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			switch k {
			case "mValue":
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					mValue = f
					hasValue = true
				}
			case "mType":
				mType = v
			case "mName":
				mName = v
			}
		}

		if hasValue && mValue >= 20.0 && mValue <= 110.0 {
			isCPU := mType == "0" || strings.Contains(strings.ToUpper(mName), "CPU")
			if isCPU {
				val := mValue
				if inHAL {
					if halCPUTemp == nil {
						halCPUTemp = &val
					}
				} else {
					if cachedCPUTemp == nil {
						cachedCPUTemp = &val
					}
				}
			}
		}
	}

	if halCPUTemp != nil {
		return halCPUTemp
	}
	return cachedCPUTemp
}

// parseBatteryTempFromThermalService 从 dumpsys thermalservice 提取电池温度备用
func parseBatteryTempFromThermalService(content string) *float64 {
	lines := strings.Split(content, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if !strings.Contains(line, "Temperature{") {
			continue
		}
		start := strings.Index(line, "Temperature{")
		end := strings.Index(line[start:], "}")
		if end == -1 {
			continue
		}
		body := line[start+len("Temperature{") : start+end]
		fields := strings.Split(body, ",")

		var mValue float64
		var mType string
		var mName string
		hasValue := false

		for _, field := range fields {
			kv := strings.SplitN(strings.TrimSpace(field), "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			switch k {
			case "mValue":
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					mValue = f
					hasValue = true
				}
			case "mType":
				mType = v
			case "mName":
				mName = v
			}
		}

		if hasValue && mValue >= 10.0 && mValue <= 80.0 {
			isBattery := mType == "2" || strings.Contains(strings.ToUpper(mName), "BATTERY")
			if isBattery {
				val := mValue
				return &val
			}
		}
	}
	return nil
}

var componentRegex = regexp.MustCompile(`([a-zA-Z0-9_\.]+)/([a-zA-Z0-9_\.\$]+)`)

// extractComponentFromLine 提取行内的 Android 组件 package 与 activity
func extractComponentFromLine(line string) (pkg string, act string) {
	matches := componentRegex.FindStringSubmatch(line)
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	return "", ""
}

// parseForegroundPkgAndActivity 从 dumpsys window 中提取当前前台焦点包名与 Activity
func parseForegroundPkgAndActivity(content string) (pkg string, activity string) {
	lines := strings.Split(content, "\n")
	var focusedPkg, focusedAct string
	var currentFocusPkg, currentFocusAct string

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.Contains(line, "mFocusedApp=") {
			p, a := extractComponentFromLine(line)
			if p != "" {
				focusedPkg, focusedAct = p, a
			}
		} else if strings.Contains(line, "mCurrentFocus=") {
			p, a := extractComponentFromLine(line)
			if p != "" {
				currentFocusPkg, currentFocusAct = p, a
			}
		}
	}

	// 优先排除系统状态栏与通知遮罩 (NotificationShade)，获取真实前台应用
	if focusedPkg != "" && focusedPkg != "com.android.systemui" {
		return focusedPkg, focusedAct
	}
	if currentFocusPkg != "" && currentFocusPkg != "com.android.systemui" {
		return currentFocusPkg, currentFocusAct
	}
	if focusedPkg != "" {
		return focusedPkg, focusedAct
	}
	return currentFocusPkg, currentFocusAct
}

// resolveAppInfo 根据 Android 包名解析中文应用名称与前端图标映射标识
func resolveAppInfo(pkg string) (name string, iconKey string) {
	if pkg == "" {
		return "待机", "standby"
	}

	appMap := map[string]struct {
		name string
		icon string
	}{
		// 系统核心组件
		"com.android.settings":                  {"系统设置", "settings"},
		"com.miui.securitycenter":               {"安全中心", "shield"},
		"com.android.launcher3":                 {"系统桌面", "launcher"},
		"com.miui.home":                         {"系统桌面", "launcher"},
		"com.bbk.launcher2":                     {"vivo桌面", "launcher"},
		"com.huawei.android.launcher":           {"华为桌面", "launcher"},
		"com.oppo.launcher":                     {"OPPO桌面", "launcher"},
		"com.sec.android.app.launcher":          {"三星桌面", "launcher"},
		"com.google.android.apps.nexuslauncher": {"Pixel桌面", "launcher"},
		"com.android.systemui":                  {"系统界面", "systemui"},
		"com.android.camera":                    {"相机", "camera"},
		"com.android.camera2":                   {"相机", "camera"},
		"com.android.gallery3d":                 {"相册", "gallery"},
		"com.miui.gallery":                      {"相册", "gallery"},
		"com.vivo.gallery":                      {"相册", "gallery"},
		"com.coloros.gallery3d":                 {"相册", "gallery"},
		"com.android.dialer":                    {"电话", "phone"},
		"com.android.contacts":                  {"联系人", "phone"},
		"com.android.mms":                       {"短信", "message"},
		"com.android.fileexplorer":              {"文件管理", "folder"},
		"com.mi.android.globalFileexplorer":     {"文件管理", "folder"},
		"com.android.browser":                   {"浏览器", "browser"},
		"com.UCMobile":                          {"UC浏览器", "browser"},
		"com.quark.browser":                     {"夸克浏览器", "browser"},
		"com.android.chrome":                    {"Chrome", "browser"},
		"com.xiaomi.market":                     {"应用商店", "market"},
		"com.bbk.appstore":                      {"vivo应用商店", "market"},
		"com.heytap.market":                     {"OPPO软件商店", "market"},
		"com.huawei.appmarket":                  {"华为应用市场", "market"},
		"com.mywebscrcpy.desktop":               {"控制台主页", "monitor"},
		"com.mywebscrcpy":                       {"控制台主页", "monitor"},

		// 社交与通讯
		"com.tencent.mm":       {"微信", "wechat"},
		"com.tencent.mobileqq": {"QQ", "qq"},
		"com.sina.weibo":       {"微博", "weibo"},
		"com.zhihu.android":    {"知乎", "zhihu"},
		"com.xingin.xhs":       {"小红书", "xiaohongshu"},

		// 短视频与影音
		"com.ss.android.ugc.aweme":      {"抖音", "douyin"},
		"com.ss.android.ugc.aweme.lite": {"抖音极速版", "douyin"},
		"com.kuaishou.nebula":           {"快手极速版", "kuaishou"},
		"com.smile.gifmaker":            {"快手", "kuaishou"},
		"tv.danmaku.bili":               {"哔哩哔哩", "bilibili"},
		"com.qiyi.video":                {"爱奇艺", "iqiyi"},
		"com.tencent.qqlive":            {"腾讯视频", "qqvideo"},
		"com.youku.phone":               {"优酷视频", "youku"},

		// 电商与本地生活
		"com.eg.android.AlipayGphone": {"支付宝", "alipay"},
		"com.taobao.taobao":           {"淘宝", "taobao"},
		"com.jingdong.app.mall":       {"京东", "jd"},
		"com.xunmeng.pinduoduo":       {"拼多多", "pinduoduo"},
		"com.sankuai.meituan":         {"美团", "meituan"},
		"com.dianping.v1":             {"大众点评", "dianping"},

		// 地图出行
		"com.autonavi.minimap": {"高德地图", "amap"},
		"com.baidu.BaiduMap":   {"百度地图", "baidumap"},

		// 音乐音频
		"com.netease.cloudmusic": {"网易云音乐", "neteasemusic"},
		"com.tencent.qqmusic":    {"QQ音乐", "qqmusic"},

		// 常用工具
		"cn.wps.moffice_eng": {"WPS Office", "wps"},
		"bin.mt.plus":        {"MT管理器", "folder"},
		"com.baidu.input_mi": {"百度输入法", "keyboard"},
	}

	if info, ok := appMap[pkg]; ok {
		return info.name, info.icon
	}

	parts := strings.Split(pkg, ".")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if len(last) > 0 {
			clean := strings.ToUpper(last[:1]) + last[1:]
			return clean, "app_default"
		}
	}

	return pkg, "app_default"
}

// parseBatteryAndThermal 解析 dumpsys battery, thermalservice, thermal_zone 及前台应用输出
func parseBatteryAndThermal(output string) TelemetryInfo {
	info := TelemetryInfo{
		UpdatedAt: time.Now().UnixMilli(),
	}

	// 0. 拆分前台应用信息
	var foregroundPart string
	if strings.Contains(output, "===FOREGROUND===") {
		fgParts := strings.SplitN(output, "===FOREGROUND===", 2)
		output = fgParts[0]
		if len(fgParts) > 1 {
			foregroundPart = fgParts[1]
		}
	}

	// 1. 拆分电池部分、系统温感服务部分及热区文件部分
	var batteryPart string
	var thermalServicePart string
	var thermalZonePart string

	if strings.Contains(output, "===THERMAL_SERVICE===") {
		p1 := strings.SplitN(output, "===THERMAL_SERVICE===", 2)
		batteryPart = p1[0]
		if len(p1) > 1 {
			if strings.Contains(p1[1], "===THERMAL_ZONE===") {
				p2 := strings.SplitN(p1[1], "===THERMAL_ZONE===", 2)
				thermalServicePart = p2[0]
				thermalZonePart = p2[1]
			} else {
				thermalServicePart = p1[1]
			}
		}
	} else if strings.Contains(output, "===THERMAL===") {
		parts := strings.SplitN(output, "===THERMAL===", 2)
		batteryPart = parts[0]
		if len(parts) > 1 {
			thermalZonePart = parts[1]
		}
	} else {
		batteryPart = output
	}

	// 2. 解析 dumpsys battery
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

	// 3. 优先通过 dumpsys thermalservice 解析 CPU 温度
	if thermalServicePart != "" {
		if cpuTemp := parseCPUTempFromThermalService(thermalServicePart); cpuTemp != nil {
			info.CPUTemp = cpuTemp
			info.CPUTempSupported = true
		}
		if info.BatteryTemp == nil {
			if batTemp := parseBatteryTempFromThermalService(thermalServicePart); batTemp != nil {
				info.BatteryTemp = batTemp
			}
		}
	}

	// 4. 降级：解析 thermal_zone (若 thermalservice 未提供 CPU 温度且存在热区数据)
	if !info.CPUTempSupported && thermalZonePart != "" {
		thermalLines := strings.Split(thermalZonePart, "\n")
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

	// 5. 解析前台应用与图标
	if foregroundPart != "" {
		pkg, _ := parseForegroundPkgAndActivity(foregroundPart)
		if pkg != "" {
			name, icon := resolveAppInfo(pkg)
			info.ForegroundPkg = pkg
			info.ForegroundAppName = name
			info.ForegroundAppIcon = icon
		}
	}

	return info
}

// updateDeviceHardwareInMemory 实时将硬件信息回写内存中的设备列表，零延迟反映至接口
func (m *Manager) updateDeviceHardwareInMemory(serial string, hw HardwareInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.devices {
		if m.devices[i].Serial == serial {
			if hw.Brand != "" {
				m.devices[i].Brand = hw.Brand
			}
			if hw.Manufacturer != "" {
				m.devices[i].Manufacturer = hw.Manufacturer
			}
			if hw.Model != "" && m.devices[i].Model == "" {
				m.devices[i].Model = hw.Model
			}
			m.devices[i].AndroidRelease = hw.AndroidRelease
			m.devices[i].SDKVersion = hw.SDKVersion
			m.devices[i].CPUABI = hw.CPUABI
			m.devices[i].BuildDisplayID = hw.BuildDisplayID
			break
		}
	}
}

// updateDeviceTelemetryInMemory 实时将遥测数据回写内存中的设备列表，零延迟反映至接口
func (m *Manager) updateDeviceTelemetryInMemory(serial string, tel TelemetryInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.devices {
		if m.devices[i].Serial == serial {
			m.devices[i].BatteryLevel = tel.BatteryLevel
			m.devices[i].BatteryTemp = tel.BatteryTemp
			m.devices[i].BatteryCharging = tel.BatteryCharging
			m.devices[i].CPUTemp = tel.CPUTemp
			m.devices[i].CPUTempSupported = tel.CPUTempSupported
			m.devices[i].ForegroundPkg = tel.ForegroundPkg
			m.devices[i].ForegroundAppName = tel.ForegroundAppName
			m.devices[i].ForegroundAppIcon = tel.ForegroundAppIcon
			m.devices[i].TelemetryUpdatedAt = tel.UpdatedAt
			break
		}
	}
}

// runHardwareFetch 异步执行 adb 命令获取设备静态硬件属性
func (m *Manager) runHardwareFetch(serial string) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	cmdStr := "getprop ro.product.brand; echo '==='; getprop ro.product.manufacturer; echo '==='; getprop ro.product.model; echo '==='; getprop ro.build.version.release; echo '==='; getprop ro.build.version.sdk; echo '==='; getprop ro.product.cpu.abi; echo '==='; getprop ro.build.display.id; exit 0"
	cmd := dependencies.CommandContext(ctx, m.adbPath, "-s", serial, "shell", cmdStr)
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	if len(out) == 0 {
		m.telemetryMu.Lock()
		delete(m.fetchingHW, serial)
		m.telemetryMu.Unlock()
		return
	}
	_ = err

	hw := parseHardwareProps(string(out))
	m.telemetryMu.Lock()
	m.hardwareCache[serial] = hw
	delete(m.fetchingHW, serial)
	m.telemetryMu.Unlock()

	m.updateDeviceHardwareInMemory(serial, hw)
}

// runTelemetryFetch 异步执行 adb 命令获取单台设备的电量、温度及当前前台应用
func (m *Manager) runTelemetryFetch(serial string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdStr := "dumpsys battery; echo '===THERMAL_SERVICE==='; dumpsys thermalservice 2>/dev/null; echo '===THERMAL_ZONE==='; for f in /sys/class/thermal/thermal_zone*/temp; do [ -f \"$f\" ] && echo \"$f:$(cat \"$f\" 2>/dev/null)\"; done; echo '===FOREGROUND==='; dumpsys window | grep -E 'mCurrentFocus|mFocusedApp'; exit 0"
	cmd := dependencies.CommandContext(ctx, m.adbPath, "-s", serial, "shell", cmdStr)
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	if len(out) == 0 {
		return
	}
	_ = err

	tel := parseBatteryAndThermal(string(out))
	m.telemetryMu.Lock()
	m.telemetryCache[serial] = tel
	m.telemetryMu.Unlock()

	m.updateDeviceTelemetryInMemory(serial, tel)
}

// telemetryLoop 后台独立守护协程，按 6 秒间隔静默轮询在线设备，零阻塞主接口
func (m *Manager) telemetryLoop() {
	// 启动后稍作延时（等待首次 refresh 完成设备识别）即刻执行首轮遥测
	select {
	case <-m.stop:
		return
	case <-time.After(800 * time.Millisecond):
		m.pollOnlineDevicesTelemetry()
	}

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
