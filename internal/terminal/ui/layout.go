package ui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

type layoutLine struct {
	text  string
	width int
}

// JoinLeft and JoinTop keep the existing alignment while measuring each ANSI
// line once and allocating the output at its final size.
func JoinLeft(blocks ...string) string {
	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		return blocks[0]
	}
	var lines []layoutLine
	maxWidth, bytes := 0, 0
	for _, block := range blocks {
		for line := range strings.SplitSeq(block, "\n") {
			w := ansi.StringWidth(line)
			maxWidth = max(maxWidth, w)
			lines = append(lines, layoutLine{line, w})
			bytes += len(line)
		}
	}
	for _, line := range lines {
		bytes += maxWidth - line.width
	}
	padding := strings.Repeat(" ", maxWidth)
	var out strings.Builder
	out.Grow(bytes + len(lines) - 1)
	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(line.text)
		out.WriteString(padding[:maxWidth-line.width])
	}
	return out.String()
}
func JoinTop(blocks ...string) string {
	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		return blocks[0]
	}
	columns := make([][]layoutLine, len(blocks))
	widths := make([]int, len(blocks))
	height, bytes, maxWidth := 0, 0, 0
	for i, block := range blocks {
		for line := range strings.SplitSeq(block, "\n") {
			w := ansi.StringWidth(line)
			widths[i] = max(widths[i], w)
			columns[i] = append(columns[i], layoutLine{line, w})
			bytes += len(line)
		}
		height = max(height, len(columns[i]))
		maxWidth = max(maxWidth, widths[i])
	}
	for i, column := range columns {
		bytes += (height - len(column)) * widths[i]
		for _, line := range column {
			bytes += widths[i] - line.width
		}
	}
	padding := strings.Repeat(" ", maxWidth)
	var out strings.Builder
	out.Grow(bytes + height - 1)
	for row := 0; row < height; row++ {
		if row > 0 {
			out.WriteByte('\n')
		}
		for col, column := range columns {
			w := 0
			if row < len(column) {
				out.WriteString(column[row].text)
				w = column[row].width
			}
			out.WriteString(padding[:widths[col]-w])
		}
	}
	return out.String()
}
