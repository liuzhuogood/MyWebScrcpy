package template

import (
	"errors"
	"strings"
	"time"
)

const (
	MethodTmCcoeffNormed = "TM_CCOEFF_NORMED"
	MethodTmSqdiffNormed = "TM_SQDIFF_NORMED"

	DefaultThreshold = 0.88
	GlobalSerial     = "global"
)

var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrInvalidImage     = errors.New("invalid image")
	ErrInvalidTemplate  = errors.New("invalid template")
	ErrTemplateExists   = errors.New("template already exists")
)

// Template represents a registered image asset.
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

// NormalizeSerial returns GlobalSerial if serial is empty or matches GlobalSerial.
func NormalizeSerial(serial string) string {
	s := strings.TrimSpace(serial)
	if s == "" || strings.EqualFold(s, GlobalSerial) {
		return GlobalSerial
	}
	return s
}
