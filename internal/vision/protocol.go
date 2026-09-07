package vision

import (
	"errors"
	"math"
	"time"
)

type Message struct {
	Type      string   `json:"type"`
	Protocol  string   `json:"protocol_version,omitempty"`
	DeviceID  string   `json:"device_id,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
	FrameID   uint64   `json:"frame_id,omitempty"`
	Timestamp int64    `json:"timestamp,omitempty"`
	Width     uint32   `json:"width,omitempty"`
	Height    uint32   `json:"height,omitempty"`
	Encoding  string   `json:"encoding,omitempty"`
	MaxFPS    int      `json:"max_fps,omitempty"`
	Objects   []Object `json:"objects,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
	Action    string   `json:"action,omitempty"`
	X         float64  `json:"x,omitempty"`
	Y         float64  `json:"y,omitempty"`
	X2        float64  `json:"x2,omitempty"`
	Y2        float64  `json:"y2,omitempty"`
	Text      string   `json:"text,omitempty"`
	Keycode   uint32   `json:"keycode,omitempty"`
	ExpiresMS int64    `json:"expires_ms,omitempty"`
}

type Object struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
}

func ValidateObjects(objects []Object) error {
	for _, obj := range objects {
		if obj.Label == "" {
			return errors.New("missing_label")
		}
		if math.IsNaN(obj.Confidence) || math.IsInf(obj.Confidence, 0) || obj.Confidence < 0 || obj.Confidence > 1 {
			return errors.New("invalid_confidence")
		}
		if !validBox(obj.X, obj.Y, obj.W, obj.H) {
			return errors.New("invalid_detection_box")
		}
	}
	return nil
}

func validBox(x, y, w, h float64) bool {
	for _, value := range []float64{x, y, w, h} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return x >= 0 && y >= 0 && w > 0 && h > 0 && x+w <= 1 && y+h <= 1
}

func IsExpired(m Message, now time.Time) bool {
	if m.ExpiresMS <= 0 || m.Timestamp <= 0 {
		return false
	}
	return now.UnixMilli()-m.Timestamp > m.ExpiresMS
}
