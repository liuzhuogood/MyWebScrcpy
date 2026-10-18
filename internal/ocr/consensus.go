package ocr

import (
	"math"
	"sort"
	"strings"
)

type boxVote struct {
	box    Box
	sample int
}
type boxCluster struct{ votes []boxVote }

func boxIoU(a, b Box) float64 {
	w := math.Max(0, math.Min(a.X+a.W, b.X+b.W)-math.Max(a.X, b.X))
	h := math.Max(0, math.Min(a.Y+a.H, b.Y+b.H)-math.Max(a.Y, b.Y))
	union := a.W*a.H + b.W*b.H - w*h
	if union <= 0 {
		return 0
	}
	return w * h / union
}

// Each source gets at most one vote per region and text. Confidence values from
// different engines are deliberately not compared when choosing a transcript.
func fuseOCRResults(results []*DetectResponse, temporal bool) *DetectResponse {
	clusters := []boxCluster{}
	for sample, res := range results {
		for _, b := range res.Boxes {
			if b.Text == "" || b.W <= 0 || b.H <= 0 {
				continue
			}
			best, score := -1, .3
			for i, c := range clusters {
				if v := boxIoU(c.votes[len(c.votes)-1].box, b); v >= score {
					best, score = i, v
				}
			}
			if best < 0 {
				clusters = append(clusters, boxCluster{})
				best = len(clusters) - 1
			}
			duplicate := false
			for _, v := range clusters[best].votes {
				if v.sample == sample {
					duplicate = true
					break
				}
			}
			if !duplicate {
				clusters[best].votes = append(clusters[best].votes, boxVote{b, sample})
			}
		}
	}
	out := &DetectResponse{Status: "not_found", Boxes: []Box{}, Samples: len(results)}
	for _, c := range clusters {
		seen := map[int]bool{}
		counts := map[string]int{}
		latest := map[string]int{}
		newestText := map[string]bool{}
		for _, v := range c.votes {
			seen[v.sample] = true
			counts[v.box.Text]++
			latest[v.box.Text] = v.sample
			if v.sample == len(results)-1 {
				newestText[v.box.Text] = true
			}
		}
		if temporal && (!seen[len(results)-1] || len(seen)*2 < len(results)) {
			continue
		}
		winner := ""
		for _, v := range c.votes {
			text := v.box.Text
			if temporal && !newestText[text] {
				continue
			}
			if winner == "" || counts[text] > counts[winner] || (counts[text] == counts[winner] && latest[text] > latest[winner]) {
				winner = text
			}
		}
		if winner == "" {
			continue
		}
		selected := []Box{}
		for _, v := range c.votes {
			if v.box.Text == winner {
				selected = append(selected, v.box)
			}
		}
		b := selected[len(selected)-1]
		median := func(f func(Box) float64) float64 {
			values := make([]float64, len(selected))
			for i, v := range selected {
				values[i] = f(v)
			}
			sort.Float64s(values)
			m := len(values) / 2
			if len(values)%2 == 0 {
				return (values[m-1] + values[m]) / 2
			}
			return values[m]
		}
		b.X = median(func(v Box) float64 { return v.X })
		b.Y = median(func(v Box) float64 { return v.Y })
		b.W = median(func(v Box) float64 { return v.W })
		b.H = median(func(v Box) float64 { return v.H })
		b.PixelX = int(math.Round(median(func(v Box) float64 { return float64(v.PixelX) })))
		b.PixelY = int(math.Round(median(func(v Box) float64 { return float64(v.PixelY) })))
		b.PixelW = int(math.Round(median(func(v Box) float64 { return float64(v.PixelW) })))
		b.PixelH = int(math.Round(median(func(v Box) float64 { return float64(v.PixelH) })))
		out.Boxes = append(out.Boxes, b)
	}
	sort.SliceStable(out.Boxes, func(i, j int) bool {
		a, b := out.Boxes[i], out.Boxes[j]
		if math.Abs(a.Y-b.Y) > math.Min(a.H, b.H)/2 {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	texts := []string{}
	for _, b := range out.Boxes {
		texts = append(texts, b.Text)
	}
	out.Text = strings.Join(texts, "\n")
	if len(out.Boxes) > 0 {
		out.Matched = true
		out.Status = "matched"
	}
	return out
}
