package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// The old column is retained only as a migration archive, never an execution fallback.
func (d *DB) migrateScriptTargets() error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query("PRAGMA table_info(markdown_scripts)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primary int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
			rows.Close()
			return err
		}
		if name == "target_devices" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if found {
		return tx.Commit()
	}
	if _, err := tx.Exec("ALTER TABLE markdown_scripts ADD COLUMN target_devices TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	rows, err = tx.Query("SELECT id, target_device, content, is_enabled FROM markdown_scripts")
	if err != nil {
		return err
	}
	type migration struct {
		id      int64
		targets []string
		enabled int
	}
	var changes []migration
	legacyField := regexp.MustCompile(`(?mi)^target_device\s*:\s*["']?([^"'\r\n]*)["']?`)
	for rows.Next() {
		var id int64
		var serial, content string
		var enabled int
		if err := rows.Scan(&id, &serial, &content, &enabled); err != nil {
			rows.Close()
			return err
		}
		targets := []string{}
		matches := legacyField.FindAllStringSubmatch(content, -1)
		valid := ValidateTargetDevices([]string{serial}) == nil && len(matches) <= 1
		if len(matches) == 1 && strings.TrimSpace(matches[0][1]) != serial {
			valid = false
		}
		if valid {
			targets = []string{serial}
		} else {
			enabled = 0
		}
		changes = append(changes, migration{id, targets, enabled})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, change := range changes {
		encoded, _ := json.Marshal(change.targets)
		_, err := tx.Exec(`UPDATE markdown_scripts SET target_devices=?, is_enabled=?, compiled_python='', compiled_version='', compiled_at=0 WHERE id=?`, string(encoded), change.enabled, change.id)
		if err != nil {
			return err
		}
		log.Printf("[DB] 脚本 %d 设备配置迁移: devices=%d pending=%t", change.id, len(change.targets), len(change.targets) == 0)
	}
	return tx.Commit()
}

func ValidateTargetDevices(targets []string) error {
	if len(targets) == 0 {
		return fmt.Errorf("请至少选择一台目标设备")
	}
	seen := make(map[string]bool, len(targets))
	for _, serial := range targets {
		if serial == "" || serial != strings.TrimSpace(serial) || serial == "auto" || serial == "global" || strings.ContainsAny(serial, "\r\n\x00") {
			return fmt.Errorf("无效目标设备序列号")
		}
		if seen[serial] {
			return fmt.Errorf("目标设备不能重复: %s", serial)
		}
		seen[serial] = true
	}
	return nil
}
