package taskrun

// OutputDef 任务产出声明项
type OutputDef struct {
	ID           string   `json:"id"`
	Category     string   `json:"category"` // report, log, screenshot, video
	Format       string   `json:"format"`   // markdown, text/utf-8, image/png, video/mp4 等
	Path         string   `json:"path"`     // 任务目录内的相对路径
	Required     bool     `json:"required"`
	AllowedTypes []string `json:"allowed_types,omitempty"` // 目录产出允许的后缀，如 [".png", ".jpg"]
}

// TaskDef 对应 task.json
type TaskDef struct {
	Name             string      `json:"name"`
	DeviceSerial     string      `json:"device_serial"`
	Goal             string      `json:"goal"`
	SuccessCondition string      `json:"success_condition"`
	TimeoutSeconds   int         `json:"timeout_seconds"`
	Source           string      `json:"source"`
	SkillDir         string      `json:"skill_dir"`
	Outputs          []OutputDef `json:"outputs"`
}

// TaskState 对应 state.json
type TaskState struct {
	Status      string  `json:"status"` // pending, running, paused, success, failed, cancelled
	Stage       string  `json:"stage"`
	Progress    float64 `json:"progress"`
	LastAction  string  `json:"last_action"`
	LastResult  string  `json:"last_result"`
	UpdatedAt   int64   `json:"updated_at"` // 毫秒时间戳
	EndedReason string  `json:"ended_reason,omitempty"`
}

// OutputCheckResult 实际产出检查结果
type OutputCheckResult struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	Format    string   `json:"format"`
	Path      string   `json:"path"`
	Required  bool     `json:"required"`
	Exists    bool     `json:"exists"`
	Valid     bool     `json:"valid"` // 非空（对于文件）或包含有效文件（对于目录）
	SizeBytes int64    `json:"size_bytes,omitempty"`
	Files     []string `json:"files,omitempty"` // 目录下的实际文件列表
	Error     string   `json:"error,omitempty"`
}

// TaskSummary 内存索引中的任务摘要
type TaskSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	DeviceSerial     string `json:"device_serial"`
	Source           string `json:"source"`
	Status           string `json:"status"`          // 来自 state.json.status
	BusinessResult   string `json:"business_result"` // 最近结果或业务状态
	OutputsComplete  bool   `json:"outputs_complete"` // 所有必填产出均有效
	Goal             string `json:"goal"`
	UpdatedAt        int64  `json:"updated_at"`
	IsStale          bool   `json:"is_stale"`
	Corrupted        bool   `json:"corrupted,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

// ActionRecord 对应 actions.jsonl 中的一条记录
type ActionRecord struct {
	Timestamp int64  `json:"timestamp"`
	Action    string `json:"action"`
	Result    string `json:"result"`
}

// TaskDetail 单个任务完整详情
type TaskDetail struct {
	ID              string              `json:"id"`
	TaskDef         TaskDef             `json:"task_def"`
	State           TaskState           `json:"state"`
	Outputs         []OutputCheckResult `json:"outputs"`
	OutputsComplete bool                `json:"outputs_complete"`
	IsStale         bool                `json:"is_stale"`
	ReportContent   string              `json:"report_content,omitempty"`
	RecentActions   []ActionRecord      `json:"recent_actions,omitempty"`
	LogTail         string              `json:"log_tail,omitempty"`
	Corrupted       bool                `json:"corrupted,omitempty"`
	ErrorMessage    string              `json:"error_message,omitempty"`
}

// ScanResult 扫描返回结果
type ScanResult struct {
	ScannedAt int64    `json:"scanned_at"`
	Count     int      `json:"count"`
	RootDir   string   `json:"root_dir"`
	Errors    []string `json:"errors,omitempty"`
}
