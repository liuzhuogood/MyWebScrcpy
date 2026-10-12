// Package taskartifact owns immutable execution evidence and artifact lifecycles.
package taskartifact

import (
	"context"
	"net/http"
	"time"
)

type RecordingOptions struct {
	Enabled     bool `json:"enabled"`
	RecordAudio bool `json:"record_audio"`
}
type ReportOptions struct {
	Enabled bool   `json:"enabled"`
	Prompt  string `json:"prompt"`
}
type Options struct {
	Recording RecordingOptions `json:"recording"`
	Report    ReportOptions    `json:"report"`
}
type Snapshot struct {
	Availability    string   `json:"availability"`
	ScriptName      string   `json:"script_name"`
	Document        string   `json:"document"`
	DocumentHash    string   `json:"document_hash"`
	CompiledHash    string   `json:"compiled_hash"`
	DocumentVersion string   `json:"document_version"`
	TargetDevices   []string `json:"target_devices"`
	Options
}
type LogState struct {
	Availability string `json:"availability"`
	CaptureState string `json:"capture_state"`
	Truncated    bool   `json:"truncated"`
	LastSeq      int64  `json:"last_seq"`
	Bytes        int64  `json:"bytes"`
	ExpiresAt    *int64 `json:"expires_at"`
	ErrorMsg     string `json:"error_msg"`
}
type RecordingState struct {
	State            string  `json:"state"`
	RecordingID      *string `json:"recording_id"`
	RecordAudio      *bool   `json:"record_audio"`
	CoverageComplete *bool   `json:"coverage_complete"`
	ErrorMsg         string  `json:"error_msg"`
}
type ReportState struct {
	State              string `json:"state"`
	GenerationRevision int    `json:"generation_revision"`
	AvailableRevision  int    `json:"available_revision"`
	CanRetry           bool   `json:"can_retry"`
	ErrorMsg           string `json:"error_msg"`
}
type Artifacts struct {
	Log       LogState       `json:"log"`
	Recording RecordingState `json:"recording"`
	Report    ReportState    `json:"report"`
}
type State struct {
	ProcessPath      string    `json:"process_path"`
	DispatchID       string    `json:"dispatch_id"`
	PreparationState string    `json:"preparation_state"`
	Options          Options   `json:"options"`
	Artifacts        Artifacts `json:"artifacts"`
}
type RecordingInfo struct {
	ID               string `json:"recording_id"`
	State            string `json:"state"`
	StartedAt        int64  `json:"started_at"`
	EndedAt          int64  `json:"ended_at"`
	CoverageComplete bool   `json:"coverage_complete"`
	ErrorMsg         string `json:"error_msg"`
}
type Recorder interface {
	StartTaskRecording(context.Context, string, string, time.Duration, bool) (RecordingInfo, error)
	StopTaskRecording(string, string, string) (RecordingInfo, error)
	TaskRecording(string, string, string) (RecordingInfo, error)
	ServeTaskRecording(http.ResponseWriter, *http.Request, string, string, string) error
}
type Citation struct {
	Seq int64 `json:"seq"`
}
type ReportOutput struct {
	Provider  string     `json:"provider"`
	Model     string     `json:"model"`
	Markdown  string     `json:"markdown"`
	Citations []Citation `json:"citations"`
}
type Summarize func(context.Context, string, string) (ReportOutput, error)
type Evidence struct {
	TaskID             string         `json:"task_id"`
	DeviceSerial       string         `json:"device_serial"`
	Status             string         `json:"status"`
	ExitCode           int            `json:"exit_code"`
	ErrorMsg           string         `json:"error_msg"`
	StartedAt          int64          `json:"started_at"`
	EndedAt            int64          `json:"ended_at"`
	DocumentHash       string         `json:"document_hash"`
	DocumentVersion    string         `json:"document_version"`
	Recording          RecordingState `json:"recording"`
	Log                LogState       `json:"log"`
	Lines              []LogLine      `json:"lines"`
	SelectedBytes      int            `json:"selected_bytes"`
	SelectionTruncated bool           `json:"selection_truncated"`
	MetadataTruncated  bool           `json:"metadata_truncated"`
}
