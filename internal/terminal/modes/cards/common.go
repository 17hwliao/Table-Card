package cards

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

const (
	reset = "\x1b[0m"
	gold  = "\x1b[38;5;220m"
	muted = "\x1b[38;5;245m"
	green = "\x1b[38;5;78m"
	red   = "\x1b[38;5;203m"
)

func decodeGame(s ui.Snapshot, target any) bool {
	return len(s.Game) != 0 && json.Unmarshal(s.Game, target) == nil
}

func title(s ui.Snapshot, name, detail string) string {
	room := s.Room.Code
	if room == "" {
		room = "练习桌"
	}
	return fmt.Sprintf("%s牌桌  /  %s%s   %s房间 %s%s\n%s", gold, name, reset, muted, room, reset, strings.Repeat("─", max(24, min(s.Width-4, 88)))) + "\n" + detail
}

func playerName(s ui.Snapshot, id string) string {
	for _, p := range s.Room.Players {
		if p.ID == id {
			return p.Name
		}
	}
	if id == s.PlayerID {
		return "你"
	}
	return id
}

func turnMark(current bool) string {
	if current {
		return gold + "▶ 行动中" + reset
	}
	return muted + "等待" + reset
}

func cardBox(label string, selected bool, color string) string {
	if selected {
		return gold + "[" + label + "]" + reset
	}
	if color != "" {
		return color + "[" + label + "]" + reset
	}
	return "[" + label + "]"
}

// handFaces draws compact card outlines, wrapping to the available terminal width.
func handFaces(labels, colors []string, selected map[int]bool, cursor, width int) string {
	var out strings.Builder
	cols := max(1, min(12, (max(30, width)-4)/11))
	for start := 0; start < len(labels); start += cols {
		end := min(start+cols, len(labels))
		for line := 0; line < 5; line++ {
			for i := start; i < end; i++ {
				tint := muted
				if i < len(colors) && colors[i] != "" {
					tint = colors[i]
				}
				if selected[i] || i == cursor {
					tint = gold
				}
				cell := ""
				switch line {
				case 0:
					if selected[i] {
						cell = "  已选中  "
					} else {
						cell = "          "
					}
				case 1:
					cell = "┌────────┐"
				case 2:
					label := ansi.Truncate(labels[i], 8, "")
					pad := 8 - ansi.StringWidth(label)
					cell = "│" + strings.Repeat(" ", pad/2) + label + strings.Repeat(" ", pad-pad/2) + "│"
				case 3:
					cell = "└────────┘"
				case 4:
					if i == cursor {
						cell = fmt.Sprintf("  ▶ %2d ◀  ", i+1)
					} else {
						cell = fmt.Sprintf("    %2d    ", i+1)
					}
				}
				out.WriteString(tint + cell + reset + " ")
			}
			out.WriteByte('\n')
		}
	}
	return out.String()
}
