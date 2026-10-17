package devicestate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"mywebscrcpy/internal/automation"
	"mywebscrcpy/internal/template"
)

// LoadedTemplate 待匹配的模板统一定义
type LoadedTemplate struct {
	Source      string // "global", "device", "skill"
	SkillID     string // 仅技能模板有效
	ID          string
	Name        string
	Threshold   float64
	Method      string
	Grayscale   bool
	Scales      []float64
	Enabled     bool
	ContentHash string // SHA-256 图像内容摘要
	Image       image.Image
}

// MetaJSON 对应技能目录下的 meta.json
type MetaJSON struct {
	Name      string    `json:"name"`
	Threshold float64   `json:"threshold"`
	Method    string    `json:"method"`
	Grayscale bool      `json:"grayscale"`
	Scales    []float64 `json:"scales"`
	Enabled   *bool     `json:"enabled"`
}

// ComputeImageHash 计算 image.Image 的 SHA-256 像素摘要
func ComputeImageHash(img image.Image) string {
	if img == nil {
		return ""
	}
	h := sha256.New()
	b := img.Bounds()
	_, _ = fmt.Fprintf(h, "bounds:%d,%d,%d,%d;", b.Min.X, b.Min.Y, b.Max.X, b.Max.Y)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			_, _ = h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(bl >> 8), byte(a >> 8)})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// LoadSkillTemplates 直接从技能目录读取 templates/<template-id>/
func LoadSkillTemplates(skillDir string) ([]LoadedTemplate, []string) {
	if strings.TrimSpace(skillDir) == "" {
		return nil, nil
	}

	tmplRoot := filepath.Join(skillDir, "templates")
	entries, err := os.ReadDir(tmplRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("读取技能模板目录失败 (%s): %v", tmplRoot, err)}
	}

	var templates []LoadedTemplate
	var errs []string

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		tmplID := entry.Name()
		dir := filepath.Join(tmplRoot, tmplID)

		metaPath := filepath.Join(dir, "meta.json")
		imgPath := filepath.Join(dir, "template.png")

		metaFile, err := os.Open(metaPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("技能模板 %s 缺少 meta.json: %v", tmplID, err))
			continue
		}
		metaData, err := io.ReadAll(io.LimitReader(metaFile, 1<<20))
		_ = metaFile.Close()
		if err != nil {
			errs = append(errs, fmt.Sprintf("技能模板 %s 读取 meta.json 失败: %v", tmplID, err))
			continue
		}

		var meta MetaJSON
		if err := json.Unmarshal(metaData, &meta); err != nil {
			errs = append(errs, fmt.Sprintf("技能模板 %s meta.json 解析失败: %v", tmplID, err))
			continue
		}

		enabled := true
		if meta.Enabled != nil {
			enabled = *meta.Enabled
		}
		if !enabled {
			continue
		}

		imgFile, err := os.Open(imgPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("技能模板 %s 缺少 template.png: %v", tmplID, err))
			continue
		}
		imgBytes, err := io.ReadAll(io.LimitReader(imgFile, 10<<20))
		_ = imgFile.Close()
		if err != nil {
			errs = append(errs, fmt.Sprintf("技能模板 %s 读取 template.png 失败: %v", tmplID, err))
			continue
		}
		img, err := png.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			errs = append(errs, fmt.Sprintf("技能模板 %s 解码 template.png 失败: %v", tmplID, err))
			continue
		}

		rawHash := sha256.Sum256(imgBytes)
		contentHash := hex.EncodeToString(rawHash[:])

		threshold := meta.Threshold
		if threshold <= 0 {
			threshold = template.DefaultThreshold
		}
		method := meta.Method
		if method == "" {
			method = template.MethodTmCcoeffNormed
		}
		scales := meta.Scales
		if len(scales) == 0 {
			scales = []float64{1.0}
		}

		name := meta.Name
		if name == "" {
			name = tmplID
		}

		templates = append(templates, LoadedTemplate{
			Source:      "skill",
			SkillID:     skillDir,
			ID:          tmplID,
			Name:        name,
			Threshold:   threshold,
			Method:      method,
			Grayscale:   meta.Grayscale,
			Scales:      scales,
			Enabled:     true,
			ContentHash: contentHash,
			Image:       img,
		})
	}

	return templates, errs
}

func scaleImage(src image.Image, scale float64) image.Image {
	if math.Abs(scale-1.0) < 1e-4 {
		return src
	}
	b := src.Bounds()
	newW := int(math.Round(float64(b.Dx()) * scale))
	newH := int(math.Round(float64(b.Dy()) * scale))
	if newW <= 0 || newH <= 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := b.Min.Y + int(float64(y)/scale)
		if srcY >= b.Max.Y {
			srcY = b.Max.Y - 1
		}
		for x := 0; x < newW; x++ {
			srcX := b.Min.X + int(float64(x)/scale)
			if srcX >= b.Max.X {
				srcX = b.Max.X - 1
			}
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func toGrayImage(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	gray := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray
}

func colorSimilarity(scene, tmpl image.Image, px, py, pw, ph int) float64 {
	tb := tmpl.Bounds()
	var sumDiff float64
	count := 0

	for dy := 0; dy < ph; dy++ {
		for dx := 0; dx < pw; dx++ {
			tc := tmpl.At(tb.Min.X+dx, tb.Min.Y+dy)
			tr, tg, tbCol, _ := tc.RGBA()
			sc := scene.At(px+dx, py+dy)
			sr, sg, sbCol, _ := sc.RGBA()

			dr := math.Abs(float64(tr>>8)-float64(sr>>8)) / 255.0
			dg := math.Abs(float64(tg>>8)-float64(sg>>8)) / 255.0
			db := math.Abs(float64(tbCol>>8)-float64(sbCol>>8)) / 255.0

			sumDiff += (dr + dg + db) / 3.0
			count++
		}
	}
	if count == 0 {
		return 1.0
	}
	return math.Max(0.0, 1.0-(sumDiff/float64(count)))
}

func matchSingleTemplate(scene image.Image, tmpl image.Image, method string, grayscale bool, threshold float64) ([]automation.MatchResult, error) {
	if method == "" {
		method = template.MethodTmCcoeffNormed
	}
	if method != template.MethodTmCcoeffNormed && method != template.MethodTmSqdiffNormed {
		return nil, fmt.Errorf("不支持的模板匹配方法: %s (仅支持 TM_CCOEFF_NORMED 或 TM_SQDIFF_NORMED)", method)
	}

	var matchScene image.Image = scene
	var matchTmpl image.Image = tmpl

	if grayscale {
		matchScene = toGrayImage(scene)
		matchTmpl = toGrayImage(tmpl)
	}

	matches, err := automation.MatchTemplatePureGo(matchScene, matchTmpl, threshold)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}

	// 若为彩色匹配 (grayscale == false)，严格核验 RGB 色彩相似度
	if !grayscale {
		var filtered []automation.MatchResult
		for _, m := range matches {
			sim := colorSimilarity(scene, tmpl, m.PixelX, m.PixelY, m.PixelW, m.PixelH)
			if sim >= threshold {
				m.Confidence = math.Min(1.0, (m.Confidence+sim)/2.0)
				filtered = append(filtered, m)
			}
		}
		return filtered, nil
	}

	return matches, nil
}

// MatchAllTemplates 对场景图执行三类模板匹配，返回匹配项及匹配过程中的错误
func MatchAllTemplates(scene image.Image, templates []LoadedTemplate, requestedIDs []string) ([]TemplateMatchItem, []string) {
	if scene == nil || len(templates) == 0 {
		return nil, nil
	}

	idFilter := make(map[string]bool)
	for _, id := range requestedIDs {
		if strings.TrimSpace(id) != "" {
			idFilter[strings.TrimSpace(id)] = true
		}
	}

	var results []TemplateMatchItem
	var matchErrors []string

	for _, tmpl := range templates {
		if !tmpl.Enabled || tmpl.Image == nil {
			continue
		}
		if len(idFilter) > 0 && !idFilter[tmpl.ID] {
			continue
		}

		scales := tmpl.Scales
		if len(scales) == 0 {
			scales = []float64{1.0}
		}

		for _, scale := range scales {
			if scale <= 0 {
				continue
			}
			scaledImg := scaleImage(tmpl.Image, scale)

			matches, err := matchSingleTemplate(scene, scaledImg, tmpl.Method, tmpl.Grayscale, tmpl.Threshold)
			if err != nil {
				matchErrors = append(matchErrors, fmt.Sprintf("模板 %s (scale: %.2f) 匹配执行失败: %v", tmpl.Name, scale, err))
				continue
			}

			for _, m := range matches {
				results = append(results, TemplateMatchItem{
					Source:       tmpl.Source,
					SkillID:      tmpl.SkillID,
					TemplateID:   tmpl.ID,
					TemplateName: tmpl.Name,
					Confidence:   m.Confidence,
					ContentHash:  tmpl.ContentHash,
					X:            m.X,
					Y:            m.Y,
					W:            m.W,
					H:            m.H,
					PixelX:       m.PixelX,
					PixelY:       m.PixelY,
					PixelW:       m.PixelW,
					PixelH:       m.PixelH,
				})
			}
		}
	}

	return results, matchErrors
}
