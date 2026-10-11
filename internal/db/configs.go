package db

import (
	"database/sql"
	"time"
)

// GetConfig 根据 key 获取配置值
func (d *DB) GetConfig(key string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var val string
	err := d.db.QueryRow("SELECT value FROM system_configs WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetConfig 设置或更新配置值
func (d *DB) SetConfig(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `INSERT INTO system_configs (key, value, updated_at) VALUES (?, ?, ?)
	          ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := d.db.Exec(query, key, value, now)
	return err
}
