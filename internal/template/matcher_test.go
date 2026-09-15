package template

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
)

// createPatternImage creates a synthetic test image with geometric features.
func createPatternImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a subtle gradient background
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := uint8((x*3 + y*2) % 100)
			img.Set(x, y, color.RGBA{R: c, G: c, B: c + 20, A: 255})
		}
	}
	return img
}

// createDistinctiveTemplate creates a 20x20 template with high-contrast shapes.
func createDistinctiveTemplate(w, h int) *image.RGBA {
	tmpl := image.NewRGBA(image.Rect(0, 0, w, h))
	// Base background
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tmpl.Set(x, y, color.RGBA{R: 40, G: 40, B: 40, A: 255})
		}
	}
	// Add cross and square inside
	for i := 4; i < w-4; i++ {
		tmpl.Set(i, h/2, color.RGBA{R: 240, G: 50, B: 50, A: 255})
		tmpl.Set(w/2, i, color.RGBA{R: 50, G: 240, B: 50, A: 255})
	}
	for y := 6; y <= 10; y++ {
		for x := 6; x <= 10; x++ {
			tmpl.Set(x, y, color.RGBA{R: 250, G: 230, B: 60, A: 255})
		}
	}
	return tmpl
}

func pasteImage(dst draw.Image, src image.Image, atX, atY int) {
	bounds := src.Bounds()
	r := image.Rect(atX, atY, atX+bounds.Dx(), atY+bounds.Dy())
	draw.Draw(dst, r, src, bounds.Min, draw.Src)
}

func TestExactMatch(t *testing.T) {
	scene := createPatternImage(100, 100)
	tmplImg := createDistinctiveTemplate(20, 20)

	targetX, targetY := 30, 40
	pasteImage(scene, tmplImg, targetX, targetY)

	tmpl := &Template{
		ID:        "tmpl-1",
		Name:      "test-target",
		Threshold: 0.88,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
		Scales:    []float64{1.0},
		Enabled:   true,
		Width:     20,
		Height:    20,
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected at least 1 match, got 0")
	}

	top := results[0]
	if top.Score < 0.99 {
		t.Errorf("Expected score near 1.0, got %f", top.Score)
	}
	if top.PixelBox.X != targetX || top.PixelBox.Y != targetY {
		t.Errorf("Expected pixel location (%d, %d), got (%d, %d)", targetX, targetY, top.PixelBox.X, top.PixelBox.Y)
	}
	if top.PixelBox.Width != 20 || top.PixelBox.Height != 20 {
		t.Errorf("Expected box size (20, 20), got (%d, %d)", top.PixelBox.Width, top.PixelBox.Height)
	}

	// Verify normalized coordinates
	expectedX := float64(targetX) / 100.0
	expectedY := float64(targetY) / 100.0
	if math.Abs(top.X-expectedX) > 1e-4 || math.Abs(top.Y-expectedY) > 1e-4 {
		t.Errorf("Normalized coords mismatch: got (%f, %f), want (%f, %f)", top.X, top.Y, expectedX, expectedY)
	}
}

func TestBrightnessOffset(t *testing.T) {
	scene := createPatternImage(100, 100)
	tmplImg := createDistinctiveTemplate(20, 20)

	targetX, targetY := 25, 35
	// Paste template with brightness offset (+35 to all channels)
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			orig := tmplImg.RGBAAt(x, y)
			bright := color.RGBA{
				R: uint8(min(255, int(orig.R)+35)),
				G: uint8(min(255, int(orig.G)+35)),
				B: uint8(min(255, int(orig.B)+35)),
				A: 255,
			}
			scene.Set(targetX+x, targetY+y, bright)
		}
	}

	tmpl := &Template{
		ID:        "tmpl-bright",
		Name:      "test-bright",
		Threshold: 0.88,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{MinScore: 0.85})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected match with brightness offset, got 0")
	}

	top := results[0]
	if top.Score < 0.95 {
		t.Errorf("Expected high correlation score under brightness shift, got %f", top.Score)
	}
	if top.PixelBox.X != targetX || top.PixelBox.Y != targetY {
		t.Errorf("Expected match at (%d, %d), got (%d, %d)", targetX, targetY, top.PixelBox.X, top.PixelBox.Y)
	}
}

func TestBackgroundNoise(t *testing.T) {
	scene := createPatternImage(100, 100)
	tmplImg := createDistinctiveTemplate(20, 20)

	targetX, targetY := 30, 40
	pasteImage(scene, tmplImg, targetX, targetY)

	// Add slight noise across the scene
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			noise := ((x*7 + y*13) % 11) - 5 // [-5, 5]
			p := scene.RGBAAt(x, y)
			r := uint8(max(0, min(255, int(p.R)+noise)))
			g := uint8(max(0, min(255, int(p.G)+noise)))
			b := uint8(max(0, min(255, int(p.B)+noise)))
			scene.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	tmpl := &Template{
		ID:        "tmpl-noise",
		Name:      "test-noise",
		Threshold: 0.85,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected match despite noise, got 0")
	}

	top := results[0]
	if top.Score < 0.90 {
		t.Errorf("Expected score > 0.90 with noise, got %f", top.Score)
	}
	if top.PixelBox.X != targetX || top.PixelBox.Y != targetY {
		t.Errorf("Expected position (%d, %d), got (%d, %d)", targetX, targetY, top.PixelBox.X, top.PixelBox.Y)
	}
}

func TestThresholdFilteringAndNoMatch(t *testing.T) {
	scene := createPatternImage(100, 100)
	tmplImg := createDistinctiveTemplate(20, 20)

	// Do NOT paste template into scene.
	tmpl := &Template{
		ID:        "tmpl-nomatch",
		Name:      "test-nomatch",
		Threshold: 0.88,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for absent template, got %d", len(results))
	}

	// Now paste template, but request impossible threshold (0.9999) when slight noise present
	pasteImage(scene, tmplImg, 30, 30)
	// introduce 1 pixel change
	scene.Set(35, 35, color.RGBA{R: 0, G: 0, B: 0, A: 255})

	resultsHighThreshold, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{MinScore: 0.9999})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if len(resultsHighThreshold) != 0 {
		t.Errorf("Expected 0 results for overly high threshold, got %d", len(resultsHighThreshold))
	}
}

func TestMultiScaleMatching(t *testing.T) {
	scene := createPatternImage(120, 120)
	origTmpl := createDistinctiveTemplate(20, 20)

	// Scale template to 1.2x (24x24) and paste
	scaledTmpl := scaleImage(origTmpl, 24, 24)
	targetX, targetY := 40, 50
	pasteImage(scene, scaledTmpl, targetX, targetY)

	tmpl := &Template{
		ID:        "tmpl-scale",
		Name:      "test-scale",
		Threshold: 0.85,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
		Scales:    []float64{0.8, 1.0, 1.2},
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, origTmpl, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected multi-scale match, got 0")
	}

	top := results[0]
	if top.Score < 0.85 {
		t.Errorf("Expected scale match score >= 0.85, got %f", top.Score)
	}

	// Allow +/- 2 pixel tolerance due to interpolation rounding
	if math.Abs(float64(top.PixelBox.X-targetX)) > 2 || math.Abs(float64(top.PixelBox.Y-targetY)) > 2 {
		t.Errorf("Expected match near (%d, %d), got (%d, %d)", targetX, targetY, top.PixelBox.X, top.PixelBox.Y)
	}
	if top.PixelBox.Width != 24 || top.PixelBox.Height != 24 {
		t.Errorf("Expected detected box (24, 24), got (%d, %d)", top.PixelBox.Width, top.PixelBox.Height)
	}
}

func TestGrayscaleAndRGBModes(t *testing.T) {
	scene := createPatternImage(100, 100)
	tmplImg := createDistinctiveTemplate(20, 20)
	pasteImage(scene, tmplImg, 20, 20)

	matcher := NewMatcher()

	// 1. Grayscale mode
	tmplGray := &Template{
		ID:        "tmpl-gray",
		Name:      "test-gray",
		Threshold: 0.88,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
	}
	resGray, err := matcher.Match(scene, tmplGray, tmplImg, MatchOptions{})
	if err != nil || len(resGray) == 0 {
		t.Fatalf("Grayscale match failed: %v, len=%d", err, len(resGray))
	}

	// 2. RGB color mode
	tmplRGB := &Template{
		ID:        "tmpl-rgb",
		Name:      "test-rgb",
		Threshold: 0.88,
		Method:    MethodTmCcoeffNormed,
		Grayscale: false,
	}
	resRGB, err := matcher.Match(scene, tmplRGB, tmplImg, MatchOptions{})
	if err != nil || len(resRGB) == 0 {
		t.Fatalf("RGB match failed: %v, len=%d", err, len(resRGB))
	}

	if resGray[0].Score < 0.99 || resRGB[0].Score < 0.99 {
		t.Errorf("Expected both scores near 1.0: gray=%f, rgb=%f", resGray[0].Score, resRGB[0].Score)
	}
}

func TestSqdiffNormed(t *testing.T) {
	scene := createPatternImage(100, 100)
	tmplImg := createDistinctiveTemplate(20, 20)
	pasteImage(scene, tmplImg, 35, 45)

	tmpl := &Template{
		ID:        "tmpl-sqdiff",
		Name:      "test-sqdiff",
		Threshold: 0.88,
		Method:    MethodTmSqdiffNormed,
		Grayscale: true,
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected match with SQDIFF_NORMED, got 0")
	}

	top := results[0]
	if top.Score < 0.98 {
		t.Errorf("Expected similarity score near 1.0 for exact SQDIFF match, got %f", top.Score)
	}
	if top.PixelBox.X != 35 || top.PixelBox.Y != 45 {
		t.Errorf("Expected match at (35, 45), got (%d, %d)", top.PixelBox.X, top.PixelBox.Y)
	}
}

func TestMultipleOccurrencesAndNMS(t *testing.T) {
	scene := createPatternImage(120, 120)
	tmplImg := createDistinctiveTemplate(20, 20)

	// Paste template at two distant locations
	pasteImage(scene, tmplImg, 15, 15)
	pasteImage(scene, tmplImg, 70, 70)

	tmpl := &Template{
		ID:        "tmpl-multi",
		Name:      "test-multi",
		Threshold: 0.88,
		Method:    MethodTmCcoeffNormed,
		Grayscale: true,
	}

	matcher := NewMatcher()
	results, err := matcher.Match(scene, tmpl, tmplImg, MatchOptions{MaxResults: 5})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected exactly 2 matches, got %d", len(results))
	}

	loc1Found := false
	loc2Found := false
	for _, res := range results {
		if res.PixelBox.X == 15 && res.PixelBox.Y == 15 {
			loc1Found = true
		}
		if res.PixelBox.X == 70 && res.PixelBox.Y == 70 {
			loc2Found = true
		}
	}

	if !loc1Found || !loc2Found {
		t.Errorf("Expected both (15, 15) and (70, 70) found, got %+v", results)
	}
}

func TestMatcherEdgeCases(t *testing.T) {
	matcher := NewMatcher()
	validImg := createPatternImage(50, 50)
	validTmplImg := createDistinctiveTemplate(20, 20)
	tmpl := &Template{ID: "t1", Threshold: 0.88}

	// 1. Nil scene or template image
	if _, err := matcher.Match(nil, tmpl, validTmplImg, MatchOptions{}); err != ErrInvalidImage {
		t.Errorf("Expected ErrInvalidImage for nil scene, got %v", err)
	}
	if _, err := matcher.Match(validImg, tmpl, nil, MatchOptions{}); err != ErrInvalidImage {
		t.Errorf("Expected ErrInvalidImage for nil template image, got %v", err)
	}
	if _, err := matcher.Match(validImg, nil, validTmplImg, MatchOptions{}); err != ErrInvalidTemplate {
		t.Errorf("Expected ErrInvalidTemplate for nil template, got %v", err)
	}

	// 2. Template larger than scene
	largeTmplImg := createPatternImage(100, 100)
	largeTmpl := &Template{ID: "t2", Threshold: 0.88}
	res, err := matcher.Match(validImg, largeTmpl, largeTmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Expected no error when template > scene, got %v", err)
	}
	if len(res) != 0 {
		t.Errorf("Expected 0 results when template > scene, got %d", len(res))
	}

	// 3. Solid color template (zero variance)
	solidTmpl := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			solidTmpl.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	solidRes, err := matcher.Match(validImg, tmpl, solidTmpl, MatchOptions{})
	if err != nil {
		t.Fatalf("Expected no error for solid template, got %v", err)
	}
	if len(solidRes) != 0 {
		t.Errorf("Expected 0 matches for zero-variance template, got %d", len(solidRes))
	}
}

func TestSceneScaleNotApplied(t *testing.T) {
	matcher := NewMatcher()

	// Base template created from a 100x100 canvas
	tmplImg := createDistinctiveTemplate(20, 20)

	// Actual scene is 200x200 (2x resolution)
	scene := createPatternImage(200, 200)
	targetImg := scaleImage(tmplImg, 40, 40)
	pasteImage(scene, targetImg, 60, 80)

	// Case 1: Template without SceneHeight - scale is 1.0, does not match 2x target
	tmplNoScene := &Template{
		ID:        "tmpl-no-scene",
		Name:      "icon",
		Threshold: 0.85,
		Scales:    []float64{1.0},
	}
	resNoScene, err := matcher.Match(scene, tmplNoScene, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if len(resNoScene) > 0 && resNoScene[0].Score >= 0.85 {
		t.Errorf("Expected scale 1.0 template not to match 2x target with >=0.85, got score %f", resNoScene[0].Score)
	}

	// Case 2: Even with SceneHeight recorded, scale stays 1.0 (no auto-rescale),
	// so it still must NOT match the 2x target.
	tmplAdaptive := &Template{
		ID:          "tmpl-adaptive",
		Name:        "icon",
		Threshold:   0.85,
		Scales:      []float64{1.0},
		SceneWidth:  100,
		SceneHeight: 100,
	}
	resAdaptive, err := matcher.Match(scene, tmplAdaptive, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if len(resAdaptive) > 0 && resAdaptive[0].Score >= 0.85 {
		t.Errorf("Expected scene fields NOT to auto-scale the template, got score %f", resAdaptive[0].Score)
	}

	// Case 3: Manual scales=[2.0] matches the 2x target without any scene metadata.
	tmplManual := &Template{
		ID:        "tmpl-manual",
		Name:      "icon",
		Threshold: 0.85,
		Scales:    []float64{2.0},
	}
	resManual, err := matcher.Match(scene, tmplManual, tmplImg, MatchOptions{})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if len(resManual) == 0 {
		t.Fatalf("Expected manual scales=[2.0] to match 2x target, got 0 matches")
	}
	if resManual[0].Score < 0.85 {
		t.Errorf("Expected score >= 0.85, got %f", resManual[0].Score)
	}
	box := resManual[0].PixelBox
	if math.Abs(float64(box.X-60)) > 2 || math.Abs(float64(box.Y-80)) > 2 {
		t.Errorf("Expected box around (60,80), got (%d, %d)", box.X, box.Y)
	}
}

