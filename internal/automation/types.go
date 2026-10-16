package automation


// Kind 表示匹配目标的类型
type Kind string

const (
	KindTemplate Kind = "template"
	KindText     Kind = "text"
	KindPixel    Kind = "pixel"
)

// MatchMode 表示多项匹配时的模式：任意命中 或 全部命中
type MatchMode string

const (
	MatchModeAny MatchMode = "any"
	MatchModeAll MatchMode = "all"
)

// Point 归一化点击坐标点
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ConnectionRequest 基础请求参数
type ConnectionRequest struct {
	Serial string `json:"serial"`
}

// TapRequest 点击请求
type TapRequest struct {
	Serial    string  `json:"serial"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	ExpiresMS int64   `json:"expires_ms,omitempty"`
	// Mode 注入模式：sdk / adb / hid，不传或传空则自动继承系统设置
	Mode     string `json:"mode,omitempty"`
	Humanize bool   `json:"humanize,omitempty"`
}

// SwipeRequest 滑动请求
type SwipeRequest struct {
	Serial     string   `json:"serial"`
	X          *float64 `json:"x,omitempty"`
	Y          *float64 `json:"y,omitempty"`
	X1         *float64 `json:"x1,omitempty"`
	Y1         *float64 `json:"y1,omitempty"`
	X2         float64  `json:"x2"`
	Y2         float64  `json:"y2"`
	ExpiresMS  int64    `json:"expires_ms,omitempty"`
	Mode       string   `json:"mode,omitempty"`
	Humanize   bool     `json:"humanize,omitempty"`
	DurationMS int64    `json:"duration_ms,omitempty"`
}

// GetStartPoint 获取划屏起点
func (r *SwipeRequest) GetStartPoint() (float64, float64, bool) {
	if r.X != nil && r.Y != nil {
		return *r.X, *r.Y, true
	}
	if r.X1 != nil && r.Y1 != nil {
		return *r.X1, *r.Y1, true
	}
	return 0, 0, false
}

// EventRequest 系统按键事件请求
type EventRequest struct {
	Serial    string `json:"serial"`
	Event     string `json:"event"` // home, back, app_switch, power, volume_up, volume_down, enter, delete
	ExpiresMS int64  `json:"expires_ms,omitempty"`
}

// SendKeyRequest 按键/文本请求
type SendKeyRequest struct {
	Serial    string   `json:"serial"`
	Text      string   `json:"text,omitempty"`
	Keycode   uint32   `json:"keycode,omitempty"`
	Keycodes  []uint32 `json:"keycodes,omitempty"`
	Modifiers []string `json:"modifiers,omitempty"`
	ExpiresMS int64    `json:"expires_ms,omitempty"`
}

// PixelPoint 像素颜色检测点定义
type PixelPoint struct {
	Name     string `json:"name,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	ColorMin [3]int  `json:"color_min"` // RGB 下限 0~255
	ColorMax [3]int  `json:"color_max"` // RGB 上限 0~255
}

// PixelPointResult 像素检测结果
type PixelPointResult struct {
	Index    int     `json:"index"`
	Name     string  `json:"name,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Pixel    [2]int  `json:"pixel"` // [px, py]
	RGB      [3]int  `json:"rgb"`
	ColorMin [3]int  `json:"color_min"`
	ColorMax [3]int  `json:"color_max"`
	Matched  bool    `json:"matched"`
}

// MatchedObject 匹配到的对象（文字或模板）
type MatchedObject struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	X          float64 `json:"x"` // 归一化坐标 0~1
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	Kind       string  `json:"kind,omitempty"` // "template" | "text"
}

// Condition 视觉判断条件
type Condition struct {
	// Template 条件
	TemplateNames []string  `json:"template_names,omitempty"`
	TemplateName  string    `json:"template_name,omitempty"`
	Threshold     float64   `json:"threshold,omitempty"`

	// Text (OCR) 条件
	Keywords       []string  `json:"keywords,omitempty"`
	Engine         string    `json:"engine,omitempty"` // auto, macocr, paddleocr, indraocr
	BBox           []float64 `json:"bbox,omitempty"`   // [x, y, w, h]
	OCRIntervalMS  int64     `json:"ocr_interval_ms,omitempty"`

	// Pixel 条件
	Points    []PixelPoint `json:"points,omitempty"`
	PixelMode string       `json:"pixel_mode,omitempty"` // all | any

	// 通用
	MatchMode MatchMode `json:"match_mode,omitempty"` // any | all
}

// ConditionRequest 状态检查请求
type ConditionRequest struct {
	Serial    string    `json:"serial"`
	Kind      Kind      `json:"kind"`
	Condition Condition `json:"condition"`
}

// CheckRequest 即时检查请求
type CheckRequest struct {
	Serial    string    `json:"serial"`
	Kind      Kind      `json:"kind"`
	Condition Condition `json:"condition"`
}

// WaitRequest 超时等待请求
type WaitRequest struct {
	Serial         string    `json:"serial"`
	Kind           Kind      `json:"kind"`
	Condition      Condition `json:"condition"`
	TimeoutMS      int64     `json:"timeout_ms,omitempty"`       // 默认 10000ms
	PollIntervalMS int64     `json:"poll_interval_ms,omitempty"` // 默认 100ms
}

// TapWhenRequest 命中后自动点击请求
type TapWhenRequest struct {
	Serial         string    `json:"serial"`
	Kind           Kind      `json:"kind"`
	Condition      Condition `json:"condition"`
	TimeoutMS      int64     `json:"timeout_ms,omitempty"`
	PollIntervalMS int64     `json:"poll_interval_ms,omitempty"`
	TargetIndex    int       `json:"target_index,omitempty"` // 多目标时取第几个（从0开始按置信度排序）
	Tap            *Point    `json:"tap,omitempty"`          // pixel 模式时必须显式指定 tap 点
	ExpiresMS      int64     `json:"expires_ms,omitempty"`
	Mode           string    `json:"mode,omitempty"`         // 注入模式，不传自动继承系统设置
	Humanize       bool      `json:"humanize,omitempty"`
}

// EvaluationResult 评估返回的详细结果
type EvaluationResult struct {
	Objects        []MatchedObject    `json:"objects,omitempty"`
	Points         []PixelPointResult `json:"points,omitempty"`
	Text           string             `json:"text,omitempty"`
	Engine         string             `json:"engine,omitempty"`
	FallbackReason []string           `json:"fallback_reason,omitempty"`
}

// CheckResponse 检查响应
type CheckResponse struct {
	Status    string           `json:"status"` // matched | not_matched
	Matched   bool             `json:"matched"`
	Kind      Kind             `json:"kind"`
	ElapsedMS int64            `json:"elapsed_ms"`
	Result    EvaluationResult `json:"result"`
}

// WaitResponse 等待响应
type WaitResponse struct {
	Status    string           `json:"status"` // matched | timeout
	Matched   bool             `json:"matched"`
	Kind      Kind             `json:"kind"`
	ElapsedMS int64            `json:"elapsed_ms"`
	Result    EvaluationResult `json:"result"`
}

// ActionResult 动作执行统一结果
type ActionResult struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"` // executed | failed
	Executed  bool   `json:"executed"`
	Mode      string `json:"mode"`
	ErrorCode string `json:"error_code,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// TapWhenResponse 条件点击响应
type TapWhenResponse struct {
	Status    string           `json:"status"` // matched | timeout
	Matched   bool             `json:"matched"`
	Kind      Kind             `json:"kind"`
	ElapsedMS int64            `json:"elapsed_ms"`
	TapPoint  *Point           `json:"tap_point,omitempty"`
	Action    *ActionResult    `json:"action,omitempty"`
	Result    EvaluationResult `json:"result"`
}

// RegionScreenshotRequest 区域截图请求
type RegionScreenshotRequest struct {
	Serial string `json:"serial"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// DeviceStatusResponse 设备状态响应
type DeviceStatusResponse struct {
	Status       string `json:"status"` // ready | offline
	Serial       string `json:"serial"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	TouchMode    string `json:"touch_mode"` // 当前系统生效的点击模式：sdk / hid / adb
	StreamActive bool   `json:"stream_active"`
}

// 常量按键映射
var NamedEvents = map[string]uint32{
	"home":        3,
	"back":        4,
	"app_switch":  187,
	"power":       26,
	"volume_up":   24,
	"volume_down": 25,
	"enter":       66,
	"delete":      67,
}
