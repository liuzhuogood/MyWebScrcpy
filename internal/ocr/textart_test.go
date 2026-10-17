package ocr

import (
	"strings"
	"testing"
)

func TestRenderBoxesTextArt(t *testing.T) {
	boxes := []Box{
		{
			Text: "Hello",
			X:    0.1,
			Y:    0.1,
		},
		{
			Text: "World",
			X:    0.5,
			Y:    0.1,
		},
		{
			Text: "中文测试",
			X:    0.2,
			Y:    0.5,
		},
	}

	art := RenderBoxesTextArt(boxes, TextArtOptions{
		Columns: 40,
		Rows:    20,
	})

	if art == "" {
		t.Fatalf("预期输出非空字符串")
	}

	if !strings.Contains(art, "Hello") || !strings.Contains(art, "World") || !strings.Contains(art, "中文测试") {
		t.Errorf("输出未包含预期内容: %s", art)
	}
}

func TestRenderBoxesTextArtEmpty(t *testing.T) {
	art := RenderBoxesTextArt(nil, TextArtOptions{})
	if art != "" {
		t.Errorf("空输入预期输出为空，实际: %q", art)
	}
}
