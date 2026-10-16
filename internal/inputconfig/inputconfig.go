package inputconfig

import (
	"encoding/json"
	"log"
	"sync"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/db"
)

// inputconfig 系统设置里的默认点击/滑动注入模式。
//
// 存 system_configs 表（key=input_config）；作为 Hub 与 Vision 动作的
// 全局默认值：请求未显式携带 mode 时用它，显式指定仍然优先，二者完全
// 向后兼容。设置页保存后调用 Set 即时生效，无需重启。

const configKey = "input_config"

// Config 注入模式配置。
type Config struct {
	// Mode 默认注入模式：sdk / adb / hid。
	Mode string `json:"mode"`
}

// Store 并发安全的配置存取。
type Store struct {
	mu sync.RWMutex
	db *db.DB

	mode string // "" 表示 sdk
}

// New 创建 Store 并加载已保存配置；database 可为 nil（无持久化模式）。
func New(database *db.DB) *Store {
	s := &Store{db: database}
	if database != nil {
		if raw, err := database.GetConfig(configKey); err == nil && raw != "" {
			var c Config
			if json.Unmarshal([]byte(raw), &c) == nil && action.ValidMode(c.Mode) {
				s.mode = c.Mode
			}
		}
	}
	return s
}

// Mode 返回生效的默认注入模式（空值归一化为 sdk）。
func (s *Store) Mode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mode == "" || s.mode == action.ModeSDK {
		return action.ModeSDK
	}
	return s.mode
}

// Set 保存并即时生效默认注入模式。
func (s *Store) Set(mode string) error {
	if !action.ValidMode(mode) || mode == "" {
		return action.ErrInvalidMode
	}
	s.mu.Lock()
	previous := s.mode
	s.mode = mode
	s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	raw, _ := json.Marshal(Config{Mode: mode})
	if err := s.db.SetConfig(configKey, string(raw)); err != nil {
		s.mu.Lock()
		s.mode = previous
		s.mu.Unlock()
		log.Printf("[input-config] 保存失败，回滚内存默认值: %v", err)
		return err
	}
	return nil
}
