package db

import (
	"context"
	"time"
)

const OperationLogRetention = 7 * 24 * time.Hour

type OperationLog struct {
	ID          string `json:"id"`
	Serial      string `json:"serial"`
	Type        string `json:"type"`
	Description string `json:"description"`
	ADBCommand  string `json:"adb_command"`
	OccurredAt  int64  `json:"occurred_at"`
}

const operationLogsSchema = `CREATE TABLE IF NOT EXISTS operation_logs (
		sequence INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT NOT NULL UNIQUE, serial TEXT NOT NULL, type TEXT NOT NULL,
		description TEXT NOT NULL, adb_command TEXT NOT NULL, occurred_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_operation_logs_time ON operation_logs(occurred_at, sequence);
	CREATE INDEX IF NOT EXISTS idx_operation_logs_device ON operation_logs(serial, occurred_at, sequence);`

func (d *DB) AppendOperationLogs(ctx context.Context, items []OperationLog) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO operation_logs
		(id, serial, type, description, adb_command, occurred_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	cutoff := time.Now().Add(-OperationLogRetention).UnixMilli()
	for _, item := range items {
		// A delayed retry must never restore a record that has already expired.
		if item.OccurredAt <= cutoff {
			continue
		}
		if _, err = stmt.ExecContext(ctx, item.ID, item.Serial, item.Type, item.Description, item.ADBCommand, item.OccurredAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// beforeSequence is an exclusive cursor; zero starts at the newest records.
func (d *DB) ListOperationLogs(ctx context.Context, serial string, beforeSequence int64, limit int) ([]OperationLog, int64, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT sequence, id, serial, type, description, adb_command, occurred_at
		FROM operation_logs WHERE occurred_at > ? AND (? = '' OR serial = ?)
		AND (? = 0 OR sequence < ?) ORDER BY sequence DESC LIMIT ?`,
		time.Now().Add(-OperationLogRetention).UnixMilli(), serial, serial, beforeSequence, beforeSequence, limit+1)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]OperationLog, 0)
	var cursor int64
	for rows.Next() {
		var item OperationLog
		var sequence int64
		if err := rows.Scan(&sequence, &item.ID, &item.Serial, &item.Type, &item.Description, &item.ADBCommand, &item.OccurredAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
		if len(items) == limit {
			cursor = sequence
		}
	}
	if len(items) > limit {
		items = items[:limit]
	} else {
		cursor = 0
	}
	return items, cursor, rows.Err()
}

func (d *DB) ClearOperationLogs(ctx context.Context, serial string) (int64, error) {
	result, err := d.db.ExecContext(ctx, `DELETE FROM operation_logs WHERE (? = '' OR serial = ?)`, serial, serial)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *DB) PruneOperationLogs(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM operation_logs WHERE occurred_at <= ?`, time.Now().Add(-OperationLogRetention).UnixMilli())
	return err
}
