package device

import (
	"testing"
)

func TestParseHardwareProps(t *testing.T) {
	raw := `Xiaomi
===
Xiaomi
===
23127PN0CC
===
14
===
34
===
arm64-v8a
===
UKQ1.230804.001`

	hw := parseHardwareProps(raw)
	if hw.Brand != "Xiaomi" {
		t.Errorf("expected Brand Xiaomi, got %s", hw.Brand)
	}
	if hw.Manufacturer != "Xiaomi" {
		t.Errorf("expected Manufacturer Xiaomi, got %s", hw.Manufacturer)
	}
	if hw.Model != "23127PN0CC" {
		t.Errorf("expected Model 23127PN0CC, got %s", hw.Model)
	}
	if hw.AndroidRelease != "14" {
		t.Errorf("expected AndroidRelease 14, got %s", hw.AndroidRelease)
	}
	if hw.SDKVersion != 34 {
		t.Errorf("expected SDKVersion 34, got %d", hw.SDKVersion)
	}
	if hw.CPUABI != "arm64-v8a" {
		t.Errorf("expected CPUABI arm64-v8a, got %s", hw.CPUABI)
	}
	if hw.BuildDisplayID != "UKQ1.230804.001" {
		t.Errorf("expected BuildDisplayID UKQ1.230804.001, got %s", hw.BuildDisplayID)
	}
}

func TestParseBatteryAndThermal_WithThermal(t *testing.T) {
	raw := `Current Battery Service state:
  AC powered: false
  USB powered: true
  Wireless powered: false
  Max charging current: 1500000
  Max charging voltage: 5000000
  Charge counter: 3500000
  status: 2
  health: 2
  present: true
  level: 85
  scale: 100
  voltage: 4200
  temperature: 365
  technology: Li-ion
===THERMAL===
/sys/class/thermal/thermal_zone0/temp:42000
/sys/class/thermal/thermal_zone1/temp:38500
`

	tel := parseBatteryAndThermal(raw)
	if tel.BatteryLevel == nil || *tel.BatteryLevel != 85 {
		t.Errorf("expected BatteryLevel 85, got %v", tel.BatteryLevel)
	}
	if tel.BatteryTemp == nil || *tel.BatteryTemp != 36.5 {
		t.Errorf("expected BatteryTemp 36.5, got %v", tel.BatteryTemp)
	}
	if tel.BatteryCharging == nil || !*tel.BatteryCharging {
		t.Errorf("expected BatteryCharging true, got %v", tel.BatteryCharging)
	}
	if !tel.CPUTempSupported {
		t.Errorf("expected CPUTempSupported true")
	}
	if tel.CPUTemp == nil || *tel.CPUTemp != 42.0 {
		t.Errorf("expected CPUTemp 42.0, got %v", tel.CPUTemp)
	}
}

func TestParseBatteryAndThermal_NoThermalGracefulFallback(t *testing.T) {
	raw := `Current Battery Service state:
  status: 3
  level: 18
  temperature: 392
===THERMAL===
cat: /sys/class/thermal/thermal_zone0/temp: Permission denied
`

	tel := parseBatteryAndThermal(raw)
	if tel.BatteryLevel == nil || *tel.BatteryLevel != 18 {
		t.Errorf("expected BatteryLevel 18, got %v", tel.BatteryLevel)
	}
	if tel.BatteryTemp == nil || *tel.BatteryTemp != 39.2 {
		t.Errorf("expected BatteryTemp 39.2, got %v", tel.BatteryTemp)
	}
	if tel.BatteryCharging == nil || *tel.BatteryCharging {
		t.Errorf("expected BatteryCharging false (discharging), got %v", tel.BatteryCharging)
	}
	if tel.CPUTempSupported {
		t.Errorf("expected CPUTempSupported false for permission denied thermal zone")
	}
	if tel.CPUTemp != nil {
		t.Errorf("expected CPUTemp nil, got %v", tel.CPUTemp)
	}
}

func TestParseBatteryAndThermal_WithThermalServiceAndForeground(t *testing.T) {
	raw := `Current Battery Service state:
  AC powered: false
  USB powered: true
  status: 2
  level: 68
  temperature: 295
===THERMAL_SERVICE===
Current temperatures from HAL:
	Temperature{mValue=35.8, mType=0, mName=CPU, mStatus=0}
	Temperature{mValue=29.1, mType=2, mName=BATTERY, mStatus=0}
===THERMAL_ZONE===
===FOREGROUND===
  mCurrentFocus=Window{3acf701 u0 com.tencent.mm/.ui.LauncherUI}
  mFocusedApp=ActivityRecord{226ce2f u0 com.tencent.mm/.ui.LauncherUI t127}
`

	tel := parseBatteryAndThermal(raw)
	if tel.BatteryLevel == nil || *tel.BatteryLevel != 68 {
		t.Errorf("expected BatteryLevel 68, got %v", tel.BatteryLevel)
	}
	if tel.BatteryTemp == nil || *tel.BatteryTemp != 29.5 {
		t.Errorf("expected BatteryTemp 29.5, got %v", tel.BatteryTemp)
	}
	if tel.CPUTemp == nil || *tel.CPUTemp != 35.8 {
		t.Errorf("expected CPUTemp 35.8, got %v", tel.CPUTemp)
	}
	if !tel.CPUTempSupported {
		t.Errorf("expected CPUTempSupported true")
	}
	if tel.ForegroundPkg != "com.tencent.mm" {
		t.Errorf("expected ForegroundPkg com.tencent.mm, got %s", tel.ForegroundPkg)
	}
	if tel.ForegroundAppName != "微信" {
		t.Errorf("expected ForegroundAppName 微信, got %s", tel.ForegroundAppName)
	}
	if tel.ForegroundAppIcon != "wechat" {
		t.Errorf("expected ForegroundAppIcon wechat, got %s", tel.ForegroundAppIcon)
	}
}

func TestParseScreenState(t *testing.T) {
	// 场景 1: 黑屏休眠
	powerAsleep := `  mWakefulness=Asleep
Display Power: state=OFF`
	keyguardShowing := `      showing=true`
	sOn, sLocked := parseScreenState(powerAsleep, keyguardShowing)
	if sOn == nil || *sOn != false {
		t.Errorf("expected screenOn false, got %v", sOn)
	}
	if sLocked == nil || *sLocked != true {
		t.Errorf("expected screenLocked true, got %v", sLocked)
	}

	// 场景 2: 亮屏已解锁
	powerAwake := `  mWakefulness=Awake
Display Power: com.android.server.power.PowerManagerService$1@3e78445`
	keyguardHidden := `      showing=false`
	sOn2, sLocked2 := parseScreenState(powerAwake, keyguardHidden)
	if sOn2 == nil || *sOn2 != true {
		t.Errorf("expected screenOn true, got %v", sOn2)
	}
	if sLocked2 == nil || *sLocked2 != false {
		t.Errorf("expected screenLocked false, got %v", sLocked2)
	}

	// 场景 3: 亮屏但在锁屏界面
	sOn3, sLocked3 := parseScreenState(powerAwake, keyguardShowing)
	if sOn3 == nil || *sOn3 != true {
		t.Errorf("expected screenOn true, got %v", sOn3)
	}
	if sLocked3 == nil || *sLocked3 != true {
		t.Errorf("expected screenLocked true, got %v", sLocked3)
	}
}

func TestParseBatteryAndThermal_WithPowerAndKeyguard(t *testing.T) {
	raw := `Current Battery Service state:
  level: 90
  temperature: 300
===THERMAL_SERVICE===
===THERMAL_ZONE===
===FOREGROUND===
  mCurrentFocus=Window{90d2071 u0 StatusBar}
===POWER===
  mWakefulness=Asleep
Display Power: state=OFF
===KEYGUARD===
      showing=true
`
	tel := parseBatteryAndThermal(raw)
	if tel.ScreenOn == nil || *tel.ScreenOn != false {
		t.Errorf("expected ScreenOn false for asleep device, got %v", tel.ScreenOn)
	}
	if tel.ScreenLocked == nil || *tel.ScreenLocked != true {
		t.Errorf("expected ScreenLocked true for asleep device, got %v", tel.ScreenLocked)
	}
}
