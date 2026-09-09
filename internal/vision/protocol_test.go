package vision

import (
	"encoding/json"
	"math"
	"testing"
)

func TestValidateObjects(t *testing.T) {
	valid := []Object{{Label: "button", Confidence: .9, X: .1, Y: .2, W: .3, H: .4}}
	if err := ValidateObjects(valid); err != nil {
		t.Fatalf("valid object rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		obj  Object
		want string
	}{
		{"missing label", Object{Confidence: .9, X: .1, Y: .1, W: .2, H: .2}, "missing_label"},
		{"confidence", Object{Label: "x", Confidence: 1.1, X: .1, Y: .1, W: .2, H: .2}, "invalid_confidence"},
		{"bounds", Object{Label: "x", Confidence: .9, X: .9, Y: .1, W: .2, H: .2}, "invalid_detection_box"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateObjects([]Object{tc.obj}); err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestValidateObjectsRejectsNonFiniteValues(t *testing.T) {
	for _, object := range []Object{
		{Label: "nan", Confidence: math.NaN(), X: .1, Y: .1, W: .2, H: .2},
		{Label: "inf", Confidence: .9, X: math.Inf(1), Y: .1, W: .2, H: .2},
	} {
		if err := ValidateObjects([]Object{object}); err == nil {
			t.Fatalf("non-finite object should be rejected: %+v", object)
		}
	}
}

func TestEmptyObjectsAreSerializedForOverlayClear(t *testing.T) {
	payload, err := json.Marshal(Message{Type: "detection.result", Objects: []Object{}})
	if err != nil {
		t.Fatalf("marshal detection result: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal detection result: %v", err)
	}
	if string(decoded["objects"]) != "[]" {
		t.Fatalf("objects = %s, want []", decoded["objects"])
	}
}
