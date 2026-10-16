package automation

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestMatchTemplatePureGo(t *testing.T) {
	// 制作一个 50x50 的底图，带有渐变纹理
	scene := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			c := uint8((x*3 + y*5) % 256)
			scene.Set(x, y, color.RGBA{R: c, G: c, B: c, A: 255})
		}
	}

	// 制作一个 10x10 的模板图，画一个明显的白色十字
	tmpl := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if x == 5 || y == 5 {
				tmpl.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				tmpl.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
			}
		}
	}

	// 将模板贴到底图的 (20, 15) 位置
	draw.Draw(scene, image.Rect(20, 15, 30, 25), tmpl, image.Point{}, draw.Src)

	// 执行模板匹配，阈值 0.90
	results, err := MatchTemplatePureGo(scene, tmpl, 0.90)
	if err != nil {
		t.Fatalf("MatchTemplatePureGo failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected at least 1 match, got 0")
	}

	match := results[0]
	if match.PixelX != 20 || match.PixelY != 15 {
		t.Errorf("expected match at (20, 15), got (%d, %d)", match.PixelX, match.PixelY)
	}

	if match.Confidence < 0.99 {
		t.Errorf("expected high confidence >= 0.99 for exact match, got %.4f", match.Confidence)
	}
}
