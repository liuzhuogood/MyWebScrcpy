package template

import (
	"errors"
	"strings"
	"time"
)

const (
	MethodTmCcoeffNormed = "TM_CCOEFF_NORMED"
	MethodTmSqdiffNormed = "TM_SQDIFF_NORMED"

	DefaultThreshold    = 0.88
	DefaultIoUThreshold = 0.35
	DefaultMaxResults   = 10
	GlobalSerial        = "global"
)

var (
	ErrTemplateNotFound   = errors.New("template not found")
	ErrTemplateNotMatched = errors.New("template not matched on screen")
	ErrInvalidImage       = errors.New("invalid image")
	ErrInvalidTemplate    = errors.New("invalid template")
	ErrTemplateExists     = errors.New("template already exists")
)

// PixelBox represents pixel coordinates and dimensions of a matched bounding box.
type PixelBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Template represents a registered image template for computer vision matching.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Serial      string    `json:"serial"` // "" or "global" denotes global template
	Threshold   float64   `json:"threshold"`
	Method      string    `json:"method"`
	Grayscale   bool      `json:"grayscale"`
	Scales      []float64 `json:"scales,omitempty"`
	Enabled     bool      `json:"enabled"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	ImageCount  int       `json:"image_count"`
	SceneWidth  int       `json:"scene_width,omitempty"`
	SceneHeight int       `json:"scene_height,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Clone returns a deep copy of the template.
func (t *Template) Clone() *Template {
	if t == nil {
		return nil
	}
	cp := *t
	if len(t.Scales) > 0 {
		cp.Scales = make([]float64, len(t.Scales))
		copy(cp.Scales, t.Scales)
	}
	return &cp
}

// MatchResult represents a single detection result of a template in a scene.
type MatchResult struct {
	TemplateID string   `json:"template_id"`
	Name       string   `json:"name"`
	Score      float64  `json:"score"`
	X          float64  `json:"x"` // normalized top-left x in [0, 1]
	Y          float64  `json:"y"` // normalized top-left y in [0, 1]
	W          float64  `json:"w"` // normalized width in [0, 1]
	H          float64  `json:"h"` // normalized height in [0, 1]
	PixelBox   PixelBox `json:"pixel_box"`
}

// DeviceStatus tracks template matching execution status for a specific device.
type DeviceStatus struct {
	Serial          string        `json:"serial"`
	Enabled         bool          `json:"enabled"`
	ActiveTemplates int           `json:"active_templates"`
	LastMatchTime   time.Time     `json:"last_match_time,omitempty"`
	LastMatches     []MatchResult `json:"last_matches,omitempty"`
}

// CreateTemplateRequest carries parameters to create a new template.
type CreateTemplateRequest struct {
	ID          string    `json:"id,omitempty"`
	Name        string    `json:"name"`
	Serial      string    `json:"serial,omitempty"`
	Threshold   float64   `json:"threshold,omitempty"`
	Method      string    `json:"method,omitempty"`
	Grayscale   *bool     `json:"grayscale,omitempty"`
	Scales      []float64 `json:"scales,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
	SceneWidth  int       `json:"scene_width,omitempty"`
	SceneHeight int       `json:"scene_height,omitempty"`
	ImageBase64 string    `json:"image_base64,omitempty"`
}

// UpdateTemplateRequest carries fields to update an existing template.
type UpdateTemplateRequest struct {
	Name        *string    `json:"name,omitempty"`
	Serial      *string    `json:"serial,omitempty"`
	Threshold   *float64   `json:"threshold,omitempty"`
	Method      *string    `json:"method,omitempty"`
	Grayscale   *bool      `json:"grayscale,omitempty"`
	Scales      *[]float64 `json:"scales,omitempty"`
	Enabled     *bool      `json:"enabled,omitempty"`
	SceneWidth  *int       `json:"scene_width,omitempty"`
	SceneHeight *int       `json:"scene_height,omitempty"`
	ImageBase64 *string    `json:"image_base64,omitempty"`
}

// MatchRequest carries parameters for detecting templates in an image.
type MatchRequest struct {
	Serial      string   `json:"serial,omitempty"`
	TemplateIDs []string `json:"template_ids,omitempty"`
	MinScore    float64  `json:"min_score,omitempty"`
	MaxResults  int      `json:"max_results,omitempty"`
	ImageBase64 string   `json:"image_base64,omitempty"`
}

// MatchOptions configures matching execution behavior.
type MatchOptions struct {
	MinScore     float64
	MaxResults   int
	IoUThreshold float64
}

// NormalizeSerial returns GlobalSerial if serial is empty or matches GlobalSerial.
func NormalizeSerial(serial string) string {
	s := strings.TrimSpace(serial)
	if s == "" || strings.EqualFold(s, GlobalSerial) {
		return GlobalSerial
	}
	return s
}

// ClickOptions configures template click execution behavior.
type ClickOptions struct {
	RandomOffset bool   `json:"random_offset"`
	Mode         string `json:"mode,omitempty"`
	Humanize     bool   `json:"humanize,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
}

// ClickRequest carries HTTP request parameters for clicking a template.
type ClickRequest struct {
	Serial       string `json:"serial"`
	Name         string `json:"name"`
	TemplateID   string `json:"template_id,omitempty"`
	RandomOffset bool   `json:"random_offset,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Humanize     bool   `json:"humanize,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
}

// ClickResult describes the outcome of a template click action.
type ClickResult struct {
	OK           bool    `json:"ok"`
	TemplateID   string  `json:"template_id"`
	TemplateName string  `json:"template_name"`
	Score        float64 `json:"score"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	RandomOffset bool    `json:"random_offset"`
	Mode         string  `json:"mode"`
	Executed     bool    `json:"executed"`
}
