package devicestate

import (
	"mywebscrcpy/internal/uipage"
)

// ScreenshotInfo 本轮截图信息
type ScreenshotInfo struct {
	DataBase64 string `json:"data_base64"` // data:image/png;base64,...
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SizeBytes  int    `json:"size_bytes"`
}

// OCRBox OCR 文本框
type OCRBox struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	PixelX     int     `json:"pixel_x"`
	PixelY     int     `json:"pixel_y"`
	PixelW     int     `json:"pixel_w"`
	PixelH     int     `json:"pixel_h"`
}

// OCRData OCR 结果摘要
type OCRData struct {
	Engine string   `json:"engine"`
	Boxes  []OCRBox `json:"boxes"`
	Text   string   `json:"text"`
	Error  string   `json:"error,omitempty"`
}

// SourceStatus 各采集源的状态与时间戳契约
type SourceStatus struct {
	Status    string `json:"status"` // "success" | "failed" | "unavailable" | "empty"
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

// TemplateMatchItem 单个模板匹配结果
type TemplateMatchItem struct {
	Source       string  `json:"source"`             // "global", "device", "skill"
	SkillID      string  `json:"skill_id,omitempty"` // 技能来源时记录技能目录/标识
	TemplateID   string  `json:"template_id"`
	TemplateName string  `json:"template_name"`
	Confidence   float64 `json:"confidence"`
	ContentHash  string  `json:"content_hash,omitempty"` // 模板图像 SHA-256 摘要
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	W            float64 `json:"w"`
	H            float64 `json:"h"`
	PixelX       int     `json:"pixel_x"`
	PixelY       int     `json:"pixel_y"`
	PixelW       int     `json:"pixel_w"`
	PixelH       int     `json:"pixel_h"`
}

// DeviceState 统一设备总状态响应
type DeviceState struct {
	ObservationID   string                  `json:"observation_id"`
	Serial          string                  `json:"serial"`
	Timestamp       int64                   `json:"timestamp"`
	DeviceInfo      map[string]any          `json:"device_info"`
	PageInfo        *uipage.PageInfo        `json:"page_info,omitempty"`
	ContextStable   bool                    `json:"context_stable"`
	Screenshot      ScreenshotInfo          `json:"screenshot"`
	OCR             OCRData                 `json:"ocr"`
	TemplateMatches []TemplateMatchItem     `json:"template_matches"`
	Sources         map[string]SourceStatus `json:"sources,omitempty"`
	PartialFailures []string                `json:"partial_failures,omitempty"`
}

// AIDecisionResult AI 决策判断结果
type AIDecisionResult struct {
	Available       bool     `json:"available"`
	ObservationID   string   `json:"observation_id"`
	PageSummary     string   `json:"page_summary,omitempty"`
	VisibleControls []string `json:"visible_controls_and_dialogs,omitempty"`
	TaskProgress    string   `json:"task_progress_assessment,omitempty"`
	NextStep        string   `json:"next_step_suggestion,omitempty"`
	Rationale       string   `json:"rationale,omitempty"`
	Uncertainties   []string `json:"uncertainties,omitempty"`
	Model           string   `json:"model,omitempty"`
	DurationMS      int64    `json:"duration_ms,omitempty"`
	Error           string   `json:"error,omitempty"`
}

// TotalStateAndDecision 总状态及决策响应
type TotalStateAndDecision struct {
	DeviceState DeviceState      `json:"device_state"`
	Decision    AIDecisionResult `json:"decision"`
}

// ServiceConfig 系统服务配置（含 AI 视觉决策服务）
type ServiceConfig struct {
	AIChatEndpoint   string `json:"ai_chat_endpoint"`
	AIModel          string `json:"ai_model"`
	AIApiKeyMasked   string `json:"ai_api_key_masked"`
	AIApiKey         string `json:"ai_api_key,omitempty"`
	AIHasKey         bool   `json:"ai_has_key"`
	AITimeoutSeconds int    `json:"ai_timeout_seconds"`
}
