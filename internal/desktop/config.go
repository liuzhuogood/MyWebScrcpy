package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	DefaultPort  = 1989
	ConfigFile   = "desktop.json"
	EndpointFile = "desktop-endpoint.json"
)

type Config struct {
	Port int `json:"port"`
}

type ConfigResponse struct {
	Port         int  `json:"port"`
	Effective    int  `json:"effective"`
	EnvPort      int  `json:"env_port"`
	IsOverridden bool `json:"is_overridden"`
}

var configMu sync.RWMutex

// ValidatePort 验证端口是否在合法有效范围内 (1-65535)
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("端口必须在 1 到 65535 之间")
	}
	return nil
}

// LoadConfig 从当前目录下的 desktop.json 读取桌面端配置
func LoadConfig() (Config, error) {
	configMu.RLock()
	defer configMu.RUnlock()

	data, err := os.ReadFile(ConfigFile)
	if os.IsNotExist(err) {
		return Config{Port: DefaultPort}, nil
	}
	if err != nil {
		return Config{Port: DefaultPort}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{Port: DefaultPort}, fmt.Errorf("解析桌面端配置失败: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	return cfg, nil
}

// SaveConfig 保存桌面配置到 desktop.json
func SaveConfig(cfg Config) error {
	if err := ValidatePort(cfg.Port); err != nil {
		return err
	}

	configMu.Lock()
	defer configMu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp(".", "desktop-*.json.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())

	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}

	return os.Rename(temp.Name(), ConfigFile)
}

// ResolvePort 解析桌面服务应该监听的端口，优先级：
// 1. 显式传参 explicit (若非空且包含有效端口)
// 2. 环境变量 MYWEBSCRCPY_DESKTOP_PORT
// 3. 环境变量 PORT
// 4. 配置文件 desktop.json 中的 port
// 5. 默认值 DefaultPort (1989)
func ResolvePort(explicit string) int {
	if explicit != "" {
		if idx := strings.LastIndex(explicit, ":"); idx >= 0 {
			if p, err := strconv.Atoi(explicit[idx+1:]); err == nil && ValidatePort(p) == nil {
				return p
			}
		} else if p, err := strconv.Atoi(explicit); err == nil && ValidatePort(p) == nil {
			return p
		}
	}

	if envPort, ok := ResolveEnvPort(); ok {
		return envPort
	}

	cfg, err := LoadConfig()
	if err == nil && ValidatePort(cfg.Port) == nil {
		return cfg.Port
	}

	return DefaultPort
}

// ResolveEnvPort 检查环境变量 MYWEBSCRCPY_DESKTOP_PORT 和 PORT
func ResolveEnvPort() (int, bool) {
	for _, envKey := range []string{"MYWEBSCRCPY_DESKTOP_PORT", "PORT"} {
		if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
			if p, err := strconv.Atoi(val); err == nil && ValidatePort(p) == nil {
				return p, true
			}
		}
	}
	return 0, false
}
