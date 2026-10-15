package taskrun

import (
	"time"

	"mywebscrcpy/internal/db"
)

const (
	HeartbeatTimeoutMs = 60000 // 60 秒未收到心跳判定为失联
	MaxLogsPerRun      = 50000 // 单个运行最大日志条数
)

type Service struct{ database *db.DB }

// TaskRunView 包装 TaskRunRecord 补充派生计算字段
type TaskRunView struct {
	db.TaskRunRecord
	Connectivity        string `json:"connectivity"` // connected, disconnected, closed
	Disconnected        bool   `json:"disconnected"`
	ConnectivityMessage string `json:"connectivity_message,omitempty"`
	DurationMs          int64  `json:"duration_ms"`
	AcknowledgedSeq     int64  `json:"acknowledged_seq,omitempty"`
}

func NewService(database *db.DB) *Service { return &Service{database: database} }

// DeriveView 计算连接状态、失联判定及耗时
func (s *Service) DeriveView(rec *db.TaskRunRecord, now int64) TaskRunView {
	if now <= 0 {
		now = time.Now().UnixMilli()
	}
	view := TaskRunView{
		TaskRunRecord: *rec,
	}

	if rec.EndedAt > 0 {
		view.DurationMs = rec.EndedAt - rec.CreatedAt
	} else {
		view.DurationMs = now - rec.CreatedAt
	}
	if view.DurationMs < 0 {
		view.DurationMs = 0
	}

	if rec.Status == "running" {
		if now-rec.LastHeartbeatAt > HeartbeatTimeoutMs {
			view.Connectivity = "disconnected"
			view.Disconnected = true
			view.ConnectivityMessage = "失联，结果未知"
		} else {
			view.Connectivity = "connected"
			view.Disconnected = false
		}
	} else {
		view.Connectivity = "closed"
		view.Disconnected = false
	}

	return view
}
