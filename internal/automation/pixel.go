package automation

import (
	"fmt"
	"image"
)

// EvaluatePixelPoints 评估屏幕图像中的一组点是否满足颜色范围
func EvaluatePixelPoints(img image.Image, points []PixelPoint, mode string) (bool, []PixelPointResult, error) {
	if len(points) == 0 {
		return false, nil, fmt.Errorf("points 列表不能为空")
	}
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "any" {
		return false, nil, fmt.Errorf("pixel_mode 必须是 all 或 any")
	}

	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()
	if imgW <= 0 || imgH <= 0 {
		return false, nil, fmt.Errorf("无效的图像尺寸")
	}

	results := make([]PixelPointResult, len(points))
	allMatched := true
	anyMatched := false

	for i, pt := range points {
		if pt.X < 0 || pt.X > 1 || pt.Y < 0 || pt.Y > 1 {
			return false, nil, fmt.Errorf("坐标超出 0~1 范围: (%.4f, %.4f)", pt.X, pt.Y)
		}

		px := int(pt.X * float64(imgW))
		py := int(pt.Y * float64(imgH))
		if px >= imgW {
			px = imgW - 1
		}
		if py >= imgH {
			py = imgH - 1
		}
		if px < 0 {
			px = 0
		}
		if py < 0 {
			py = 0
		}

		c := img.At(bounds.Min.X+px, bounds.Min.Y+py)
		r, g, b, _ := c.RGBA()
		// RGBA() 返回 0~65535，右移 8 位转为 0~255
		r8 := int(r >> 8)
		g8 := int(g >> 8)
		b8 := int(b >> 8)

		matched := (r8 >= pt.ColorMin[0] && r8 <= pt.ColorMax[0]) &&
			(g8 >= pt.ColorMin[1] && g8 <= pt.ColorMax[1]) &&
			(b8 >= pt.ColorMin[2] && b8 <= pt.ColorMax[2])

		results[i] = PixelPointResult{
			Index:    i,
			Name:     pt.Name,
			X:        pt.X,
			Y:        pt.Y,
			Pixel:    [2]int{px, py},
			RGB:      [3]int{r8, g8, b8},
			ColorMin: pt.ColorMin,
			ColorMax: pt.ColorMax,
			Matched:  matched,
		}

		if matched {
			anyMatched = true
		} else {
			allMatched = false
		}
	}

	overallMatched := allMatched
	if mode == "any" {
		overallMatched = anyMatched
	}

	return overallMatched, results, nil
}
