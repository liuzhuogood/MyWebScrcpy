package automation

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"sync"
)

// MatchResult 模板匹配到的单一目标
type MatchResult struct {
	Confidence float64
	X          float64 // 归一化 X (0~1)
	Y          float64 // 归一化 Y (0~1)
	W          float64 // 归一化 W (0~1)
	H          float64 // 归一化 H (0~1)
	PixelX     int
	PixelY     int
	PixelW     int
	PixelH     int
}

// toGrayArray 将 image.Image 转换为一维连续灰度数组及尺寸
func toGrayArray(img image.Image) ([]float64, int, int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	gray := make([]float64, w*h)

	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			r8 := float64(r >> 8)
			g8 := float64(g >> 8)
			b8 := float64(b >> 8)
			gray[idx] = 0.299*r8 + 0.587*g8 + 0.114*b8
			idx++
		}
	}
	return gray, w, h
}

// computeIntegralImages 计算灰度图的和与平方和积分图
func computeIntegralImages(gray []float64, w, h int) ([]float64, []float64) {
	stride := w + 1
	sum := make([]float64, (w+1)*(h+1))
	sqSum := make([]float64, (w+1)*(h+1))

	for y := 0; y < h; y++ {
		var rowSum, rowSqSum float64
		for x := 0; x < w; x++ {
			val := gray[y*w+x]
			rowSum += val
			rowSqSum += val * val

			sum[(y+1)*stride+(x+1)] = sum[y*stride+(x+1)] + rowSum
			sqSum[(y+1)*stride+(x+1)] = sqSum[y*stride+(x+1)] + rowSqSum
		}
	}
	return sum, sqSum
}

// windowSum 利用积分图获取任意矩形区域 [x, y, x+w, y+h] 的和
func windowSum(integral []float64, stride, x, y, w, h int) float64 {
	x1, y1 := x, y
	x2, y2 := x+w, y+h
	return integral[y2*stride+x2] - integral[y1*stride+x2] - integral[y2*stride+x1] + integral[y1*stride+x1]
}

// calcNCCAt 计算指定位置 (x, y) 的归一化相关系数
func calcNCCAt(
	x, y int,
	sceneGray []float64,
	sceneW int,
	tmplNorm []float64,
	tmplW, tmplH int,
	tmplCount, tmplStd float64,
	sumIntegral, sqIntegral []float64,
	intStride int,
) float64 {
	iSum := windowSum(sumIntegral, intStride, x, y, tmplW, tmplH)
	iSqSum := windowSum(sqIntegral, intStride, x, y, tmplW, tmplH)

	iMean := iSum / tmplCount
	iVar := iSqSum - iSum*iMean
	if iVar <= 1e-6 {
		return -1.0
	}
	iStd := math.Sqrt(iVar)

	var crossCorr float64
	for ty := 0; ty < tmplH; ty++ {
		rowScene := sceneGray[(y+ty)*sceneW+x : (y+ty)*sceneW+x+tmplW]
		rowTmpl := tmplNorm[ty*tmplW : ty*tmplW+tmplW]
		for k := 0; k < tmplW; k++ {
			crossCorr += rowScene[k] * rowTmpl[k]
		}
	}

	return crossCorr / (tmplStd * iStd)
}

// MatchTemplatePureGo 纯 Go 归一化相关系数（NCC / TM_CCOEFF_NORMED）匹配
// 支持多核并发加速和大图自适应步长粗搜 + 精搜
func MatchTemplatePureGo(scene image.Image, tmpl image.Image, threshold float64) ([]MatchResult, error) {
	if scene == nil || tmpl == nil {
		return nil, fmt.Errorf("图像或模板不能为空")
	}

	sceneGray, sceneW, sceneH := toGrayArray(scene)
	tmplGray, tmplW, tmplH := toGrayArray(tmpl)

	if tmplW > sceneW || tmplH > sceneH {
		return nil, fmt.Errorf("模板尺寸 (%dx%d) 大于原图尺寸 (%dx%d)", tmplW, tmplH, sceneW, sceneH)
	}
	if tmplW <= 0 || tmplH <= 0 {
		return nil, fmt.Errorf("模板尺寸无效")
	}

	tmplCount := float64(tmplW * tmplH)
	var tmplSum, tmplSqSum float64
	for _, v := range tmplGray {
		tmplSum += v
		tmplSqSum += v * v
	}
	tmplMean := tmplSum / tmplCount
	tmplVar := tmplSqSum - tmplSum*tmplMean
	if tmplVar <= 1e-6 {
		return nil, fmt.Errorf("模板图像对比度过低或为纯色")
	}
	tmplStd := math.Sqrt(tmplVar)

	tmplNorm := make([]float64, len(tmplGray))
	for i, v := range tmplGray {
		tmplNorm[i] = v - tmplMean
	}

	sumIntegral, sqIntegral := computeIntegralImages(sceneGray, sceneW, sceneH)
	intStride := sceneW + 1

	outW := sceneW - tmplW + 1
	outH := sceneH - tmplH + 1

	// 自适应粗搜步长：大图步长取 2~4，小图精确步长 1
	stride := 1
	if sceneW >= 800 || sceneH >= 800 {
		stride = 2
		if sceneW >= 1500 || sceneH >= 1500 {
			stride = 3
		}
	}

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	type bestCoord struct {
		conf float64
		x, y int
	}

	workerResults := make([]bestCoord, numWorkers)
	var wg sync.WaitGroup

	rowsPerWorker := (outH + numWorkers - 1) / numWorkers

	for wIdx := 0; wIdx < numWorkers; wIdx++ {
		startRow := wIdx * rowsPerWorker
		endRow := startRow + rowsPerWorker
		if endRow > outH {
			endRow = outH
		}
		if startRow >= outH {
			continue
		}

		wg.Add(1)
		go func(workerID, yStart, yEnd int) {
			defer wg.Done()
			localBest := bestCoord{conf: -1.0, x: -1, y: -1}

			for y := yStart; y < yEnd; y += stride {
				for x := 0; x < outW; x += stride {
					ncc := calcNCCAt(x, y, sceneGray, sceneW, tmplNorm, tmplW, tmplH, tmplCount, tmplStd, sumIntegral, sqIntegral, intStride)
					if ncc > localBest.conf {
						localBest.conf = ncc
						localBest.x = x
						localBest.y = y
					}
				}
			}
			workerResults[workerID] = localBest
		}(wIdx, startRow, endRow)
	}
	wg.Wait()

	globalBest := bestCoord{conf: -1.0, x: -1, y: -1}
	for _, res := range workerResults {
		if res.conf > globalBest.conf {
			globalBest = res
		}
	}

	// 如果步长 > 1，在粗搜命中的最佳点周围进行精细搜索 (stride=1)
	if stride > 1 && globalBest.x >= 0 && globalBest.y >= 0 && globalBest.conf >= (threshold-0.12) {
		fxStart := int(math.Max(0, float64(globalBest.x-stride)))
		fxEnd := int(math.Min(float64(outW-1), float64(globalBest.x+stride)))
		fyStart := int(math.Max(0, float64(globalBest.y-stride)))
		fyEnd := int(math.Min(float64(outH-1), float64(globalBest.y+stride)))

		for fy := fyStart; fy <= fyEnd; fy++ {
			for fx := fxStart; fx <= fxEnd; fx++ {
				ncc := calcNCCAt(fx, fy, sceneGray, sceneW, tmplNorm, tmplW, tmplH, tmplCount, tmplStd, sumIntegral, sqIntegral, intStride)
				if ncc > globalBest.conf {
					globalBest.conf = ncc
					globalBest.x = fx
					globalBest.y = fy
				}
			}
		}
	}

	if globalBest.conf >= threshold && globalBest.x >= 0 && globalBest.y >= 0 {
		result := MatchResult{
			Confidence: math.Min(1.0, math.Max(0.0, globalBest.conf)),
			X:          float64(globalBest.x) / float64(sceneW),
			Y:          float64(globalBest.y) / float64(sceneH),
			W:          float64(tmplW) / float64(sceneW),
			H:          float64(tmplH) / float64(sceneH),
			PixelX:     globalBest.x,
			PixelY:     globalBest.y,
			PixelW:     tmplW,
			PixelH:     tmplH,
		}
		return []MatchResult{result}, nil
	}

	return nil, nil
}
