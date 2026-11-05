package device

import (
	"context"
	"fmt"
	"mywebscrcpy/internal/dependencies"
	"time"
)

type ScreenState struct {
	ScreenOn     *bool  `json:"screenOn"`
	ScreenLocked *bool  `json:"screenLocked"`
	Error        string `json:"error,omitempty"`
}

func (m *Manager) ReadScreenState(ctx context.Context, serial string, includeLock bool) ScreenState {
	ctx, cancel := context.WithTimeout(dependencies.BackgroundADB(ctx), 5*time.Second)
	defer cancel()
	power, err := dependencies.CommandContext(ctx, m.adbPath, "-s", serial, "shell", "dumpsys", "power").Output()
	if err != nil {
		return ScreenState{Error: fmt.Sprintf("读取屏幕电源状态失败: %v", err)}
	}
	on, _ := parseScreenState(string(power), "")
	if on == nil {
		return ScreenState{Error: "设备未返回可识别的屏幕电源状态"}
	}
	result := ScreenState{ScreenOn: on}
	if includeLock {
		policy, err := dependencies.CommandContext(ctx, m.adbPath, "-s", serial, "shell", "dumpsys", "window", "policy").Output()
		if err != nil {
			result.Error = fmt.Sprintf("读取锁屏状态失败: %v", err)
			return result
		}
		_, result.ScreenLocked = parseScreenState(string(power), string(policy))
	}
	return result
}
