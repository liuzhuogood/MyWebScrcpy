package template

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sort"
	"sync"
)

// imageMatrix holds flattened float64 pixel data for 1 or more channels.
type imageMatrix struct {
	width    int
	height   int
	channels [][]float64 // 1 channel for grayscale, 3 channels for RGB
}

// candidateMatch stores an unsuppressed candidate position and score.
type candidateMatch struct {
	Score float64
	X     int
	Y     int
	W     int
	H     int
}

// SceneContext caches preprocessed image matrices and integral images for reuse.
type SceneContext struct {
	Width    int
	Height   int
	GrayMat  *imageMatrix
	RGBMat   *imageMatrix
	GrayII   []float64
	GrayII2  []float64
	RGBII    [][]float64
	RGBII2   [][]float64
	mu       sync.Mutex
}

// Matcher implements pure-Go OpenCV-compatible template matching algorithms.
type Matcher struct{}

// NewMatcher creates a new Matcher instance.
func NewMatcher() *Matcher {
	return &Matcher{}
}

// PrepareScene pre-processes a scene image, caching matrix conversions and integral images.
func (m *Matcher) PrepareScene(scene image.Image) (*SceneContext, error) {
	if scene == nil {
		return nil, ErrInvalidImage
	}
	bounds := scene.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, ErrInvalidImage
	}

	return &SceneContext{
		Width:  w,
		Height: h,
	}, nil
}

// getGray returns the grayscale matrix and integral images, computing them lazily.
func (sc *SceneContext) getGray(scene image.Image) (*imageMatrix, []float64, []float64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.GrayMat != nil {
		return sc.GrayMat, sc.GrayII, sc.GrayII2
	}

	mat := toGrayMatrix(scene)
	sc.GrayMat = &mat
	ii, ii2 := computeIntegralImages(mat.channels, mat.width, mat.height)
	sc.GrayII = ii[0]
	sc.GrayII2 = ii2[0]
	return sc.GrayMat, sc.GrayII, sc.GrayII2
}

// getRGB returns the RGB matrix and integral images, computing them lazily.
func (sc *SceneContext) getRGB(scene image.Image) (*imageMatrix, [][]float64, [][]float64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.RGBMat != nil {
		return sc.RGBMat, sc.RGBII, sc.RGBII2
	}

	mat := toRGBMatrix(scene)
	sc.RGBMat = &mat
	sc.RGBII, sc.RGBII2 = computeIntegralImages(mat.channels, mat.width, mat.height)
	return sc.RGBMat, sc.RGBII, sc.RGBII2
}

// Match searches for template occurrences in the scene image.
func (m *Matcher) Match(scene image.Image, tmpl *Template, tmplImg image.Image, opts MatchOptions) ([]MatchResult, error) {
	if scene == nil || tmplImg == nil {
		return nil, ErrInvalidImage
	}
	if tmpl == nil {
		return nil, ErrInvalidTemplate
	}

	sc, err := m.PrepareScene(scene)
	if err != nil {
		return nil, err
	}

	return m.MatchWithScene(sc, scene, tmpl, tmplImg, opts)
}

// MatchOne finds the single best match result. Returns nil if no match reaches threshold.
func (m *Matcher) MatchOne(scene image.Image, tmpl *Template, tmplImg image.Image) (*MatchResult, error) {
	opts := MatchOptions{
		MinScore:     0,
		MaxResults:   1,
		IoUThreshold: DefaultIoUThreshold,
	}
	results, err := m.Match(scene, tmpl, tmplImg, opts)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// MatchWithScene searches for template occurrences using a pre-initialized SceneContext.
func (m *Matcher) MatchWithScene(sc *SceneContext, scene image.Image, tmpl *Template, tmplImg image.Image, opts MatchOptions) ([]MatchResult, error) {
	if sc == nil || scene == nil || tmplImg == nil {
		return nil, ErrInvalidImage
	}
	if tmpl == nil {
		return nil, ErrInvalidTemplate
	}

	tBounds := tmplImg.Bounds()
	tOrigW, tOrigH := tBounds.Dx(), tBounds.Dy()
	if tOrigW <= 0 || tOrigH <= 0 {
		return nil, ErrInvalidImage
	}

	threshold := tmpl.Threshold
	if opts.MinScore > 0 {
		threshold = opts.MinScore
	}
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	iouThreshold := opts.IoUThreshold
	if iouThreshold <= 0 {
		iouThreshold = DefaultIoUThreshold
	}

	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = DefaultMaxResults
	}

	scales := tmpl.Scales
	if len(scales) == 0 {
		scales = []float64{1.0}
	}

	method := tmpl.Method
	if method == "" {
		method = MethodTmCcoeffNormed
	}

	var allCandidates []candidateMatch

	for _, scale := range scales {
		if scale <= 0 {
			continue
		}
		effectiveScale := scale

		var scaledTmplImg image.Image
		var tw, th int
		if math.Abs(effectiveScale-1.0) < 1e-4 {
			scaledTmplImg = tmplImg
			tw, th = tOrigW, tOrigH
		} else {
			tw = int(math.Round(float64(tOrigW) * effectiveScale))
			th = int(math.Round(float64(tOrigH) * effectiveScale))
			if tw < 2 || th < 2 || tw > sc.Width || th > sc.Height {
				continue
			}
			scaledTmplImg = scaleImage(tmplImg, tw, th)
		}

		if tw > sc.Width || th > sc.Height {
			continue
		}

		var cands []candidateMatch
		if tmpl.Grayscale {
			srcMat, ii, ii2 := sc.getGray(scene)
			tmplMat := toGrayMatrix(scaledTmplImg)
			cands = matchSingleScale(srcMat, ii, ii2, &tmplMat, method, threshold)
		} else {
			srcMat, ii, ii2 := sc.getRGB(scene)
			tmplMat := toRGBMatrix(scaledTmplImg)
			cands = matchSingleScaleMultiChannel(srcMat, ii, ii2, &tmplMat, method, threshold)
		}

		allCandidates = append(allCandidates, cands...)
	}

	if len(allCandidates) == 0 {
		return nil, nil
	}

	// Sort candidates by score descending
	sort.Slice(allCandidates, func(i, j int) bool {
		return allCandidates[i].Score > allCandidates[j].Score
	})

	// Non-Maximum Suppression (NMS)
	var kept []candidateMatch
	for _, cand := range allCandidates {
		suppressed := false
		for _, prev := range kept {
			if computeIoU(cand, prev) > iouThreshold {
				suppressed = true
				break
			}
		}
		if !suppressed {
			kept = append(kept, cand)
			if len(kept) >= maxResults {
				break
			}
		}
	}

	invW := 1.0 / float64(sc.Width)
	invH := 1.0 / float64(sc.Height)

	results := make([]MatchResult, len(kept))
	for i, c := range kept {
		results[i] = MatchResult{
			TemplateID: tmpl.ID,
			Name:       tmpl.Name,
			Score:      math.Round(c.Score*10000) / 10000,
			X:          float64(c.X) * invW,
			Y:          float64(c.Y) * invH,
			W:          float64(c.W) * invW,
			H:          float64(c.H) * invH,
			PixelBox: PixelBox{
				X:      c.X,
				Y:      c.Y,
				Width:  c.W,
				Height: c.H,
			},
		}
	}

	return results, nil
}

// matchSingleScale handles single-channel (grayscale) matching.
func matchSingleScale(
	src *imageMatrix,
	ii, ii2 []float64,
	tmpl *imageMatrix,
	method string,
	threshold float64,
) []candidateMatch {
	sw, sh := src.width, src.height
	tw, th := tmpl.width, tmpl.height
	outW := sw - tw + 1
	outH := sh - th + 1
	if outW <= 0 || outH <= 0 {
		return nil
	}

	n := float64(tw * th)
	tPix := tmpl.channels[0]
	var tSum, tSum2 float64
	for _, v := range tPix {
		tSum += v
		tSum2 += v * v
	}

	meanT := tSum / n
	var varT float64
	tPrime := make([]float64, len(tPix))
	for i, v := range tPix {
		diff := v - meanT
		tPrime[i] = diff
		varT += diff * diff
	}

	if method == MethodTmCcoeffNormed && varT <= 1e-12 {
		return nil
	}

	scores := make([]float64, outW*outH)
	stride := sw + 1
	sPix := src.channels[0]

	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > outH {
		numWorkers = outH
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	chunkSize := (outH + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for wIdx := 0; wIdx < numWorkers; wIdx++ {
		startY := wIdx * chunkSize
		endY := (wIdx + 1) * chunkSize
		if endY > outH {
			endY = outH
		}

		go func(startY, endY int) {
			defer wg.Done()
			for y := startY; y < endY; y++ {
				rowOffset := y * outW
				for x := 0; x < outW; x++ {
					// Window sums from integral image
					top := y * stride
					bot := (y + th) * stride
					s := ii[bot+x+tw] - ii[top+x+tw] - ii[bot+x] + ii[top+x]
					s2 := ii2[bot+x+tw] - ii2[top+x+tw] - ii2[bot+x] + ii2[top+x]

					if method == MethodTmCcoeffNormed {
						varI := s2 - (s*s)/n
						if varI <= 1e-12 {
							scores[rowOffset+x] = -1.0
							continue
						}

						denom := math.Sqrt(varT * varI)
						var num float64
						for dy := 0; dy < th; dy++ {
							tRow := tPrime[dy*tw : (dy+1)*tw]
							iRow := sPix[(y+dy)*sw+x : (y+dy)*sw+x+tw]
							for dx := 0; dx < tw; dx++ {
								num += tRow[dx] * iRow[dx]
							}
						}

						score := num / denom
						if score > 1.0 {
							score = 1.0
						} else if score < -1.0 {
							score = -1.0
						}
						scores[rowOffset+x] = score
					} else {
						// TM_SQDIFF_NORMED
						denom := math.Sqrt(tSum2 * s2)
						if denom <= 1e-12 {
							scores[rowOffset+x] = 0.0
							continue
						}

						var crossCorr float64
						for dy := 0; dy < th; dy++ {
							tRow := tPix[dy*tw : (dy+1)*tw]
							iRow := sPix[(y+dy)*sw+x : (y+dy)*sw+x+tw]
							for dx := 0; dx < tw; dx++ {
								crossCorr += tRow[dx] * iRow[dx]
							}
						}

						diff := (tSum2 + s2 - 2.0*crossCorr) / denom
						if diff < 0 {
							diff = 0
						} else if diff > 1.0 {
							diff = 1.0
						}
						scores[rowOffset+x] = 1.0 - diff
					}
				}
			}
		}(startY, endY)
	}
	wg.Wait()

	return extractPeaks(scores, outW, outH, tw, th, threshold)
}

// matchSingleScaleMultiChannel handles multi-channel (RGB) matching.
func matchSingleScaleMultiChannel(
	src *imageMatrix,
	ii, ii2 [][]float64,
	tmpl *imageMatrix,
	method string,
	threshold float64,
) []candidateMatch {
	sw, sh := src.width, src.height
	tw, th := tmpl.width, tmpl.height
	outW := sw - tw + 1
	outH := sh - th + 1
	if outW <= 0 || outH <= 0 {
		return nil
	}

	numChannels := len(src.channels)
	n := float64(tw * th)

	tPrimes := make([][]float64, numChannels)
	var varTSum float64
	var tSum2Total float64

	for c := 0; c < numChannels; c++ {
		tPix := tmpl.channels[c]
		var tSum, tSum2 float64
		for _, v := range tPix {
			tSum += v
			tSum2 += v * v
		}
		tSum2Total += tSum2

		meanT := tSum / n
		tPrime := make([]float64, len(tPix))
		for i, v := range tPix {
			diff := v - meanT
			tPrime[i] = diff
			varTSum += diff * diff
		}
		tPrimes[c] = tPrime
	}

	if method == MethodTmCcoeffNormed && varTSum <= 1e-12 {
		return nil
	}

	scores := make([]float64, outW*outH)
	stride := sw + 1

	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > outH {
		numWorkers = outH
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	chunkSize := (outH + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for wIdx := 0; wIdx < numWorkers; wIdx++ {
		startY := wIdx * chunkSize
		endY := (wIdx + 1) * chunkSize
		if endY > outH {
			endY = outH
		}

		go func(startY, endY int) {
			defer wg.Done()
			for y := startY; y < endY; y++ {
				rowOffset := y * outW
				for x := 0; x < outW; x++ {
					top := y * stride
					bot := (y + th) * stride

					if method == MethodTmCcoeffNormed {
						var varISum float64
						for c := 0; c < numChannels; c++ {
							s := ii[c][bot+x+tw] - ii[c][top+x+tw] - ii[c][bot+x] + ii[c][top+x]
							s2 := ii2[c][bot+x+tw] - ii2[c][top+x+tw] - ii2[c][bot+x] + ii2[c][top+x]
							v := s2 - (s*s)/n
							if v > 0 {
								varISum += v
							}
						}

						if varISum <= 1e-12 {
							scores[rowOffset+x] = -1.0
							continue
						}

						denom := math.Sqrt(varTSum * varISum)
						var num float64
						for c := 0; c < numChannels; c++ {
							tP := tPrimes[c]
							sP := src.channels[c]
							for dy := 0; dy < th; dy++ {
								tRow := tP[dy*tw : (dy+1)*tw]
								iRow := sP[(y+dy)*sw+x : (y+dy)*sw+x+tw]
								for dx := 0; dx < tw; dx++ {
									num += tRow[dx] * iRow[dx]
								}
							}
						}

						score := num / denom
						if score > 1.0 {
							score = 1.0
						} else if score < -1.0 {
							score = -1.0
						}
						scores[rowOffset+x] = score
					} else {
						var s2Sum float64
						for c := 0; c < numChannels; c++ {
							s2 := ii2[c][bot+x+tw] - ii2[c][top+x+tw] - ii2[c][bot+x] + ii2[c][top+x]
							s2Sum += s2
						}

						denom := math.Sqrt(tSum2Total * s2Sum)
						if denom <= 1e-12 {
							scores[rowOffset+x] = 0.0
							continue
						}

						var crossCorr float64
						for c := 0; c < numChannels; c++ {
							tPix := tmpl.channels[c]
							sPix := src.channels[c]
							for dy := 0; dy < th; dy++ {
								tRow := tPix[dy*tw : (dy+1)*tw]
								iRow := sPix[(y+dy)*sw+x : (y+dy)*sw+x+tw]
								for dx := 0; dx < tw; dx++ {
									crossCorr += tRow[dx] * iRow[dx]
								}
							}
						}

						diff := (tSum2Total + s2Sum - 2.0*crossCorr) / denom
						if diff < 0 {
							diff = 0
						} else if diff > 1.0 {
							diff = 1.0
						}
						scores[rowOffset+x] = 1.0 - diff
					}
				}
			}
		}(startY, endY)
	}
	wg.Wait()

	return extractPeaks(scores, outW, outH, tw, th, threshold)
}

// extractPeaks identifies local maxima in the score matrix that meet the threshold.
func extractPeaks(scores []float64, outW, outH, tw, th int, threshold float64) []candidateMatch {
	var cands []candidateMatch
	for y := 0; y < outH; y++ {
		rowOffset := y * outW
		for x := 0; x < outW; x++ {
			score := scores[rowOffset+x]
			if score < threshold {
				continue
			}

			isPeak := true
			for dy := -1; dy <= 1 && isPeak; dy++ {
				ny := y + dy
				if ny < 0 || ny >= outH {
					continue
				}
				nOffset := ny * outW
				for dx := -1; dx <= 1; dx++ {
					nx := x + dx
					if nx < 0 || nx >= outW || (dx == 0 && dy == 0) {
						continue
					}
					neighborScore := scores[nOffset+nx]
					// Tie-breaking: strict > for prior scanline positions, >= for subsequent
					if neighborScore > score || (neighborScore == score && (dy < 0 || (dy == 0 && dx < 0))) {
						isPeak = false
						break
					}
				}
			}

			if isPeak {
				cands = append(cands, candidateMatch{
					Score: score,
					X:     x,
					Y:     y,
					W:     tw,
					H:     th,
				})
			}
		}
	}
	return cands
}

// computeIntegralImages builds integral images and squared integral images for all channels.
func computeIntegralImages(channels [][]float64, w, h int) ([][]float64, [][]float64) {
	stride := w + 1
	totalSize := stride * (h + 1)
	numChannels := len(channels)

	iis := make([][]float64, numChannels)
	ii2s := make([][]float64, numChannels)

	for c := 0; c < numChannels; c++ {
		ii := make([]float64, totalSize)
		ii2 := make([]float64, totalSize)
		pix := channels[c]

		for y := 0; y < h; y++ {
			var rowSum, rowSum2 float64
			pOffset := y * w
			currRow := (y + 1) * stride
			prevRow := y * stride

			for x := 0; x < w; x++ {
				val := pix[pOffset+x]
				rowSum += val
				rowSum2 += val * val

				col := x + 1
				ii[currRow+col] = ii[prevRow+col] + rowSum
				ii2[currRow+col] = ii2[prevRow+col] + rowSum2
			}
		}

		iis[c] = ii
		ii2s[c] = ii2
	}

	return iis, ii2s
}

// toGrayMatrix converts any image.Image to a single-channel float64 matrix.
func toGrayMatrix(img image.Image) imageMatrix {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	data := make([]float64, w*h)

	switch im := img.(type) {
	case *image.Gray:
		for y := 0; y < h; y++ {
			rowStart := (bounds.Min.Y + y) * im.Stride
			for x := 0; x < w; x++ {
				data[y*w+x] = float64(im.Pix[rowStart+bounds.Min.X+x])
			}
		}
	case *image.RGBA:
		for y := 0; y < h; y++ {
			rowStart := (bounds.Min.Y + y) * im.Stride
			for x := 0; x < w; x++ {
				p := rowStart + (bounds.Min.X+x)*4
				r := float64(im.Pix[p])
				g := float64(im.Pix[p+1])
				b := float64(im.Pix[p+2])
				data[y*w+x] = 0.299*r + 0.587*g + 0.114*b
			}
		}
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			rowStart := (bounds.Min.Y + y) * im.Stride
			for x := 0; x < w; x++ {
				p := rowStart + (bounds.Min.X+x)*4
				r := float64(im.Pix[p])
				g := float64(im.Pix[p+1])
				b := float64(im.Pix[p+2])
				data[y*w+x] = 0.299*r + 0.587*g + 0.114*b
			}
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				// RGBA returns values in [0, 0xffff]
				data[y*w+x] = (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 257.0
			}
		}
	}

	return imageMatrix{
		width:    w,
		height:   h,
		channels: [][]float64{data},
	}
}

// toRGBMatrix converts an image.Image to 3 channels (R, G, B) of float64 data.
func toRGBMatrix(img image.Image) imageMatrix {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	total := w * h
	rChan := make([]float64, total)
	gChan := make([]float64, total)
	bChan := make([]float64, total)

	switch im := img.(type) {
	case *image.RGBA:
		for y := 0; y < h; y++ {
			rowStart := (bounds.Min.Y + y) * im.Stride
			for x := 0; x < w; x++ {
				p := rowStart + (bounds.Min.X+x)*4
				idx := y*w + x
				rChan[idx] = float64(im.Pix[p])
				gChan[idx] = float64(im.Pix[p+1])
				bChan[idx] = float64(im.Pix[p+2])
			}
		}
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			rowStart := (bounds.Min.Y + y) * im.Stride
			for x := 0; x < w; x++ {
				p := rowStart + (bounds.Min.X+x)*4
				idx := y*w + x
				rChan[idx] = float64(im.Pix[p])
				gChan[idx] = float64(im.Pix[p+1])
				bChan[idx] = float64(im.Pix[p+2])
			}
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				idx := y*w + x
				rChan[idx] = float64(r) / 257.0
				gChan[idx] = float64(g) / 257.0
				bChan[idx] = float64(b) / 257.0
			}
		}
	}

	return imageMatrix{
		width:    w,
		height:   h,
		channels: [][]float64{rChan, gChan, bChan},
	}
}

// scaleImage scales an image using bilinear interpolation.
func scaleImage(src image.Image, targetW, targetH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	scaleX := float64(srcW) / float64(targetW)
	scaleY := float64(srcH) / float64(targetH)

	for y := 0; y < targetH; y++ {
		srcY := (float64(y)+0.5)*scaleY - 0.5
		if srcY < 0 {
			srcY = 0
		}
		if srcY > float64(srcH-1) {
			srcY = float64(srcH - 1)
		}
		y0 := int(srcY)
		y1 := y0 + 1
		if y1 >= srcH {
			y1 = srcH - 1
		}
		fy := srcY - float64(y0)

		for x := 0; x < targetW; x++ {
			srcX := (float64(x)+0.5)*scaleX - 0.5
			if srcX < 0 {
				srcX = 0
			}
			if srcX > float64(srcW-1) {
				srcX = float64(srcW - 1)
			}
			x0 := int(srcX)
			x1 := x0 + 1
			if x1 >= srcW {
				x1 = srcW - 1
			}
			fx := srcX - float64(x0)

			c00 := src.At(bounds.Min.X+x0, bounds.Min.Y+y0)
			c10 := src.At(bounds.Min.X+x1, bounds.Min.Y+y0)
			c01 := src.At(bounds.Min.X+x0, bounds.Min.Y+y1)
			c11 := src.At(bounds.Min.X+x1, bounds.Min.Y+y1)

			r00, g00, b00, a00 := c00.RGBA()
			r10, g10, b10, a10 := c10.RGBA()
			r01, g01, b01, a01 := c01.RGBA()
			r11, g11, b11, a11 := c11.RGBA()

			w00 := (1.0 - fx) * (1.0 - fy)
			w10 := fx * (1.0 - fy)
			w01 := (1.0 - fx) * fy
			w11 := fx * fy

			r := uint8((w00*float64(r00) + w10*float64(r10) + w01*float64(r01) + w11*float64(r11)) / 257.0)
			g := uint8((w00*float64(g00) + w10*float64(g10) + w01*float64(g01) + w11*float64(g11)) / 257.0)
			b := uint8((w00*float64(b00) + w10*float64(b10) + w01*float64(b01) + w11*float64(b11)) / 257.0)
			a := uint8((w00*float64(a00) + w10*float64(a10) + w01*float64(a01) + w11*float64(a11)) / 257.0)

			dst.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return dst
}

// computeIoU computes Intersection over Union between two candidate bounding boxes.
func computeIoU(a, b candidateMatch) float64 {
	x1 := max(a.X, b.X)
	y1 := max(a.Y, b.Y)
	x2 := min(a.X+a.W, b.X+b.W)
	y2 := min(a.Y+a.H, b.Y+b.H)

	w := max(0, x2-x1)
	h := max(0, y2-y1)
	interArea := w * h
	if interArea <= 0 {
		return 0
	}

	unionArea := a.W*a.H + b.W*b.H - interArea
	if unionArea <= 0 {
		return 0
	}

	return float64(interArea) / float64(unionArea)
}
