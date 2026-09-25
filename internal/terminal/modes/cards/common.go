package cards

import (
	"encoding/json"
	"fmt"
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
