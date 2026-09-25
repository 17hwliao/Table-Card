package cards

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

type liarCard struct {
	ID   uint8 `json:"id"`
	Rank uint8 `json:"rank"`
}
type liarView struct {
	Round   int   `json:"round"`
	Target  uint8 `json:"target"`
	Turn    int   `json:"turn"`
	Phase   uint8 `json:"phase"`
	Players []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Alive     bool   `json:"alive"`
		Shots     int    `json:"shots"`
		HandCount int    `json:"handCount"`
	} `json:"players"`
	Pending *struct {
		Seat    int  `json:"seat"`
		Count   int  `json:"count"`
		CanCall bool `json:"canCall"`
	} `json:"pending"`
	Hand          []liarCard `json:"hand"`
	LastChallenge *struct {
		Bluff      bool       `json:"bluff"`
		Shooter    int        `json:"shooter"`
		ShotNumber int        `json:"shotNumber"`
		Revealed   []liarCard `json:"revealed"`
	} `json:"lastChallenge"`
	Winner string `json:"winner"`
}

type LiarBar struct{ selected map[int]bool }

func NewLiarBar() *LiarBar        { return &LiarBar{} }
func (*LiarBar) Mode() table.Mode { return table.LiarBarMode }
func (c *LiarBar) Reset()         { c.selected = make(map[int]bool) }

func (c *LiarBar) View(s ui.Snapshot) string {
	var v liarView
	if !decodeGame(s, &v) {
		return "骗子酒馆状态正在同步…"
	}
	if c.selected == nil {
		c.Reset()
	}
	var b strings.Builder
	fmt.Fprintln(&b, title(s, "骗子酒馆", fmt.Sprintf("第 %d 轮   本局牌：%s", v.Round, liarRank(v.Target))))
	fmt.Fprintln(&b)
	for i, p := range v.Players {
		life := green + "存活" + reset
		if !p.Alive {
			life = red + "已淘汰" + reset
		}
		fmt.Fprintf(&b, "%d. %-12s 手牌 %d   空枪 %d/6   %s", i+1, p.Name, p.HandCount, p.Shots, life)
		if i == v.Turn {
			fmt.Fprintf(&b, "   %s", turnMark(true))
		}
		fmt.Fprintln(&b)
	}
	if v.Pending != nil {
		fmt.Fprintf(&b, "\n桌面盖牌：%d 张", v.Pending.Count)
		fmt.Fprintf(&b, "\n最近出牌：%s 宣称打出 %d 张 %s\n", v.Players[v.Pending.Seat].Name, v.Pending.Count, liarRank(v.Target))
	} else {
		fmt.Fprintln(&b, "\n桌面尚无待质疑的暗牌")
	}
	if v.LastChallenge != nil {
		verdict := green + "质疑成立：上一手是谎言" + reset
		if !v.LastChallenge.Bluff {
			verdict = red + "质疑失败：上一手符合宣称" + reset
		}
		fmt.Fprintf(&b, "%s\n翻牌：", verdict)
		for _, card := range v.LastChallenge.Revealed {
			fmt.Fprintf(&b, " %s", liarRank(card.Rank))
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "\n%s你的手牌（数字切换选择）%s\n", gold, reset)
	if len(v.Hand) == 0 {
		fmt.Fprintln(&b, "（没有手牌，只能质疑上一手）")
	}
	for i, card := range v.Hand {
		fmt.Fprintf(&b, "%s%2d %s%s   ", selectedMark(c.selected[i]), i+1, cardBox(liarRank(card.Rank), c.selected[i], ""), reset)
		if (i+1)%8 == 0 {
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b)
	if v.Winner != "" {
		fmt.Fprintf(&b, "\n%s游戏结束，获胜者：%s%s\n", gold, playerName(s, v.Winner), reset)
	}
	if s.PlayerID == playerAt(v.Players, v.Turn) && v.Phase == 0 {
		if v.Pending != nil {
			fmt.Fprintln(&b, "轮到你：选择 1–3 张暗牌后按 Enter 跟牌，或按 C 质疑")
		} else {
			fmt.Fprintln(&b, "轮到你：选择 1–3 张牌后按 Enter 盖牌")
		}
	} else if v.Phase == 0 {
		fmt.Fprintf(&b, "%s等待 %s 行动%s\n", muted, playerName(s, playerAt(v.Players, v.Turn)), reset)
	}
	fmt.Fprintln(&b, "操作：1-9 选择 · Enter 盖牌 · C 质疑 · Esc 清空选择")
	return b.String()
}

func (c *LiarBar) Key(s ui.Snapshot, key string) ui.Result {
	var v liarView
	if !decodeGame(s, &v) {
		return ui.Result{}
	}
	if c.selected == nil {
		c.Reset()
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "esc" || key == "escape" {
		c.Reset()
		return ui.Result{Handled: true, Status: "已清空选牌"}
	}
	if v.Phase != 0 || s.PlayerID != playerAt(v.Players, v.Turn) {
		return ui.Result{}
	}
	if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(v.Hand) {
		if c.selected[n-1] {
			delete(c.selected, n-1)
		} else {
			if len(c.selected) >= 3 {
				return ui.Result{Handled: true, Status: "每次最多选择 3 张牌"}
			}
			c.selected[n-1] = true
		}
		return ui.Result{Handled: true, Status: fmt.Sprintf("已选择 %d 张（最多 3 张）", len(c.selected))}
	}
	if key == "c" || key == "l" {
		if v.Pending == nil {
			return ui.Result{Handled: true, Status: "当前没有可质疑的暗牌"}
		}
		c.Reset()
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "challenge"}), Status: "提出质疑"}
	}
	if key == "enter" || key == "return" {
		if v.Pending != nil && len(c.selected) == 0 {
			return ui.Result{Handled: true, Status: "选择 1–3 张牌跟牌，或按 C 质疑"}
		}
		if len(c.selected) < 1 || len(c.selected) > 3 {
			return ui.Result{Handled: true, Status: "每次必须选择 1–3 张牌"}
		}
		ids := make([]uint8, 0, len(c.selected))
		for i, card := range v.Hand {
			if c.selected[i] {
				ids = append(ids, card.ID)
			}
		}
		c.Reset()
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "play", "cardIds": ids}), Status: fmt.Sprintf("暗出 %d 张 %s", len(ids), liarRank(v.Target))}
	}
	return ui.Result{}
}

func liarRank(rank uint8) string {
	switch rank {
	case 1:
		return "Q"
	case 2:
		return "K"
	case 3:
		return "A"
	case 4:
		return "Joker"
	default:
		return "?"
	}
}
func playerAt(players []struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Alive     bool   `json:"alive"`
	Shots     int    `json:"shots"`
	HandCount int    `json:"handCount"`
}, seat int) string {
	if seat >= 0 && seat < len(players) {
		return players[seat].ID
	}
	return ""
}
func selectedMark(selected bool) string {
	if selected {
		return gold + ">" + reset
	}
	return " "
}
