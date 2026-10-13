package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrGenerationStale = errors.New("生成期间任务已修改，请重新生成")

type ScriptGeneration struct {
	SourceHash      string `json:"source_hash"`
	CodeHash        string `json:"code_hash"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ContractVersion string `json:"contract_version"`
	GeneratedAt     int64  `json:"generated_at"`
}

func (d *DB) migrateGenerations() error {
	_, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS script_generations (
        script_id INTEGER PRIMARY KEY, source_hash TEXT NOT NULL, code_hash TEXT NOT NULL,
        provider TEXT NOT NULL, model TEXT NOT NULL, contract_version TEXT NOT NULL,
        generated_at INTEGER NOT NULL
    )`)
	return err
}

func (d *DB) GetScriptGeneration(id int64) (*ScriptGeneration, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var item ScriptGeneration
	err := d.db.QueryRow(`SELECT source_hash, code_hash, provider, model, contract_version, generated_at FROM script_generations WHERE script_id=?`, id).
		Scan(&item.SourceHash, &item.CodeHash, &item.Provider, &item.Model, &item.ContractVersion, &item.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// SaveScriptGeneration compares the frozen revision and writes code/provenance atomically.
func (d *DB) SaveScriptGeneration(ctx context.Context, snapshot MarkdownScriptRecord, python string, source ScriptGeneration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	result, err := tx.ExecContext(ctx, `UPDATE markdown_scripts SET compiled_version=?, compiled_python=?, compiled_at=?
        WHERE id=? AND updated_at=? AND content=? AND name=?`, snapshot.ExtractedVersion, python, now, snapshot.ID, snapshot.UpdatedAt, snapshot.Content, snapshot.Name)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrGenerationStale
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO script_generations(script_id, source_hash, code_hash, provider, model, contract_version, generated_at)
        VALUES(?,?,?,?,?,?,?) ON CONFLICT(script_id) DO UPDATE SET source_hash=excluded.source_hash,
        code_hash=excluded.code_hash, provider=excluded.provider, model=excluded.model,
        contract_version=excluded.contract_version, generated_at=excluded.generated_at`,
		snapshot.ID, source.SourceHash, source.CodeHash, source.Provider, source.Model, source.ContractVersion, now)
	if err != nil {
		return fmt.Errorf("保存生成来源失败: %w", err)
	}
	return tx.Commit()
}
