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
