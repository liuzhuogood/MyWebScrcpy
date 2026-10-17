package ocr

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// TextArtOptions 文字排版画渲染选项
type TextArtOptions struct {
	Columns int // 目标半角列数，默认 90
	Rows    int // 目标行数，0 表示自适应
}

// runeWidth 返回字符在等宽终端下的显示宽度（ASCII / 半角为 1，CJK 全角等为 2）
func runeWidth(r rune) int {
	if unicode.Is(unicode.Scripts["Han"], r) ||
		unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo) {
		return 2
	}
	// 全角 ASCII 及特殊标点符号范围
	if r >= 0xFF01 && r <= 0xFF60 || r >= 0xFFE0 && r <= 0xFFE6 {
		return 2
	}
	// 中文常见全角符号（如《》【】、。，“”‘’）
	switch r {
	case '《', '》', '【', '】', '、', '。', '，', '：', '；', '？', '！', '…', '—', '·', '“', '”', '‘', '’':
		return 2
	}
	return 1
}

// RenderBoxesTextArt 将 OCR 返回的文字框按归一化坐标渲染为纯文本布局画
func RenderBoxesTextArt(boxes []Box, opts TextArtOptions) string {
	if len(boxes) == 0 {
		return ""
	}

	cols := opts.Columns
	if cols <= 0 {
		cols = 90
	}

	// 找出纵横比或覆盖范围
	rows := opts.Rows
	if rows <= 0 {
		// 估算行数：手机屏幕通常宽高比约为 9:16 ~ 9:20
		// 终端字符高度通常约为宽度的 2 倍，因此等效行数约等于 (cols * (屏幕H / 屏幕W) * 0.5)
		// 默认取 80~100 行
		rows = 90
	}

	// grid[r][c] 存储每个单元格的 rune，使用 2*cols 的半角宽度空间
	totalWidth := cols * 2
	grid := make([][]rune, rows)
	for i := range grid {
		row := make([]rune, totalWidth)
		for j := range row {
			row[j] = ' '
		}
		grid[i] = row
	}

	// 按纵坐标 Y、横坐标 X 排序
	sortedBoxes := make([]Box, len(boxes))
	copy(sortedBoxes, boxes)
	sort.SliceStable(sortedBoxes, func(i, j int) bool {
		yi := int(sortedBoxes[i].Y * float64(rows))
		yj := int(sortedBoxes[j].Y * float64(rows))
		if yi != yj {
			return yi < yj
		}
		return sortedBoxes[i].X < sortedBoxes[j].X
	})

	for _, b := range sortedBoxes {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}

		r := int(math.Round(b.Y * float64(rows-1)))
		if r < 0 {
			r = 0
		} else if r >= rows {
			r = rows - 1
		}

		c0 := int(math.Round(b.X * float64(totalWidth-1)))
		if c0 < 0 {
			c0 = 0
		} else if c0 >= totalWidth {
			c0 = totalWidth - 1
		}

		currR := r
		currC := c0

		for _, ch := range text {
			if ch == '\r' {
				continue
			}
			if ch == '\n' {
				currR++
				currC = c0
				continue
			}
			w := runeWidth(ch)
			if currR < 0 || currR >= rows || currC < 0 || currC+w > totalWidth {
				currC += w
				continue
			}

			grid[currR][currC] = ch
			// 若为全角字符，后一格占位（为空格或继续）
			if w == 2 && currC+1 < totalWidth {
				grid[currR][currC+1] = ' '
			}
			currC += w
		}
	}

	// 转为字符串，并去除首尾空白行
	lines := make([]string, 0, rows)
	for _, row := range grid {
		var sb strings.Builder
		for i := 0; i < len(row); i++ {
			r := row[i]
			if r == 0 {
				continue
			}
			sb.WriteRune(r)
			if runeWidth(r) == 2 {
				// 跳过全角字符占用的第二格
				i++
			}
		}
		lines = append(lines, strings.TrimRightFunc(sb.String(), unicode.IsSpace))
	}

	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n")
}
