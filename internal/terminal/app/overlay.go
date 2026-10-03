package app

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func (m *Model) overlayKey(key string) {
	switch key {
	case "esc", "enter", "f1":
		m.overlay = ""
		m.overlayOffset = 0
	case "up", "[":
		m.overlayOffset = max(0, m.overlayOffset-1)
	case "down", "]":
		m.overlayOffset++
	case "pgup":
		m.overlayOffset = max(0, m.overlayOffset-max(1, m.height-8))
	case "pgdown":
		m.overlayOffset += max(1, m.height-8)
	}
}
func (m *Model) overlayView() string {
	if m.overlayLast != m.overlay {
		m.overlayLast = m.overlay
		m.overlayOffset = 0
	}
	width := max(10, m.width-8)
	visible := max(1, m.height-6)
	lines := strings.Split(ansi.Wrap(m.overlay, width, ""), "\n")
	m.overlayOffset = min(m.overlayOffset, max(0, len(lines)-visible))
	end := min(len(lines), m.overlayOffset+visible)
	body := strings.Join(lines[m.overlayOffset:end], "\n")
	body += "\n\n" + muted.Render(ansi.Truncate(fmt.Sprintf("↑↓ / PgUp PgDn 阅读 %d–%d/%d · Esc / Enter 返回", m.overlayOffset+1, end, len(lines)), width, "…"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}
