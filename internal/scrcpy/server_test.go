package scrcpy

import "testing"

func TestQualityPercentToMaxSize(t *testing.T) {
	tests := []struct {
		percent int
		want    int
	}{
		{percent: 0, want: 0},
		{percent: 50, want: 540},
		{percent: 80, want: 864},
		{percent: 100, want: 0},
		{percent: 120, want: 0},
	}

	for _, tt := range tests {
		if got := QualityPercentToMaxSize(tt.percent); got != tt.want {
			t.Fatalf("QualityPercentToMaxSize(%d) = %d, want %d", tt.percent, got, tt.want)
		}
	}
}
