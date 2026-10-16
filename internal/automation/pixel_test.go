package automation

import (
	"image"
	"image/color"
	"testing"
)

func TestEvaluatePixelPoints(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// (10, 10) 设为纯红 (255, 0, 0)
	img.Set(10, 10, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	// (50, 50) 设为纯绿 (0, 255, 0)
	img.Set(50, 50, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	points := []PixelPoint{
		{
			Name:     "red_point",
			X:        0.10,
			Y:        0.10,
			ColorMin: [3]int{200, 0, 0},
			ColorMax: [3]int{255, 50, 50},
		},
		{
			Name:     "green_point",
			X:        0.50,
			Y:        0.50,
			ColorMin: [3]int{0, 200, 0},
			ColorMax: [3]int{50, 255, 50},
		},
	}

	// 1. 全部匹配模式 (all)
	matched, res, err := EvaluatePixelPoints(img, points, "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true, got false")
	}
	if len(res) != 2 || !res[0].Matched || !res[1].Matched {
		t.Errorf("expected all points to match, got %+v", res)
	}

	// 2. 混合一个不匹配的点
	pointsWithFail := append(points, PixelPoint{
		Name:     "blue_point_fail",
		X:        0.50,
		Y:        0.50,
		ColorMin: [3]int{0, 0, 200},
		ColorMax: [3]int{50, 50, 255},
	})

	matchedAll, _, err := EvaluatePixelPoints(img, pointsWithFail, "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matchedAll {
		t.Errorf("expected matchedAll=false when one point fails")
	}

	matchedAny, _, err := EvaluatePixelPoints(img, pointsWithFail, "any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matchedAny {
		t.Errorf("expected matchedAny=true when at least one matches")
	}
}
