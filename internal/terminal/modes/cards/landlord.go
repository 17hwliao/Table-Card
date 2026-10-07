package cards

import (
	"fmt"
	"strconv"
	"strings"

	gamecards "github.com/17hwliao/table-card-independent/internal/cards"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

type landlordView struct {
	Phase      uint8            `json:"phase"`
	TurnSeat   int              `json:"turnSeat"`
	TurnPlayer string           `json:"turnPlayer"`
	Landlord   int              `json:"landlord"`
	HighestBid int              `json:"highestBid"`
	Multiplier int              `json:"multiplier"`
	Winner     int              `json:"winner"`
	CardsLeft  []int            `json:"cardsLeft"`
	Hand       []gamecards.Card `json:"hand"`
	Bottom     []gamecards.Card `json:"bottom"`
	Trick      *struct {
		Seat  int              `json:"Seat"`
		Cards []gamecards.Card `json:"Cards"`
	} `json:"trick"`
}

type Landlord struct {
	selected map[int]bool
	cursor   int
}

func NewLandlord() *Landlord       { return &Landlord{} }
func (*Landlord) Mode() table.Mode { return table.LandlordMode }
func (c *Landlord) Reset()         { c.selected = make(map[int]bool); c.cursor = 0 }

func (c *Landlord) View(s ui.Snapshot) string {
	var v landlordView
	if !decodeGame(s, &v) {
		return "斗地主牌局状态正在同步…"
	}
	if c.selected == nil {
		c.Reset()
	}
	var b strings.Builder
	fmt.Fprintln(&b, title(s, "斗地主", fmt.Sprintf("叫分 %d   倍数 ×%d", v.HighestBid, max(v.Multiplier, 1))))
	fmt.Fprintln(&b)
	for i, p := range s.Room.Players {
		role := "农民"
		if i == v.Landlord {
			role = "地主"
		}
		left := 0
		if i < len(v.CardsLeft) {
			left = v.CardsLeft[i]
		}
		fmt.Fprintf(&b, "%-14s %-6s 手牌 %2d 张   %s\n", p.Name, role, left, turnMark(p.ID == v.TurnPlayer))
	}
	fmt.Fprintln(&b)
	if v.Phase == 0 {
		fmt.Fprintf(&b, "叫地主阶段   当前最高叫分：%d\n", v.HighestBid)
	} else if v.Phase == 3 {
		team := "农民队"
		landlordWon := v.Winner == v.Landlord
		if landlordWon {
			team = "地主"
		}
		fmt.Fprintf(&b, "%s本局结束，%s 获胜 · 最终倍数 ×%d%s\n", gold, team, max(1, v.Multiplier), reset)
		for seat, player := range s.Room.Players {
			points := max(1, v.Multiplier)
			if seat == v.Landlord {
				points *= 2
			}
			if landlordWon != (seat == v.Landlord) {
				points = -points
			}
			fmt.Fprintf(&b, "  %s  %+d 分\n", player.Name, points)
		}
	} else {
		fmt.Fprintf(&b, "当前行动：%s\n", playerName(s, v.TurnPlayer))
		if v.Trick == nil || len(v.Trick.Cards) == 0 {
			fmt.Fprintln(&b, "桌面：新一轮，等待领出")
		} else {
			played := make([]string, len(v.Trick.Cards))
			for i, card := range v.Trick.Cards {
				played[i] = card.String()
			}
			fmt.Fprintf(&b, "桌面：%s 出牌  %s\n", playerName(s, seatPlayerID(s, v.Trick.Seat)), strings.Join(played, " "))
		}
		if len(v.Bottom) > 0 {
			parts := make([]string, len(v.Bottom))
			for i, card := range v.Bottom {
				parts[i] = card.String()
			}
			fmt.Fprintf(&b, "底牌：%s\n", strings.Join(parts, " "))
		}
	}
	fmt.Fprintf(&b, "\n%s你的手牌 · ← → 移动 / Space 选中%s\n", gold, reset)
	labels := make([]string, len(v.Hand))
	colors := make([]string, len(v.Hand))
	for i, card := range v.Hand {
		labels[i] = card.String()
		if card.Suit == gamecards.Heart || card.Suit == gamecards.Diamond {
			colors[i] = red
		}
	}
	fmt.Fprintln(&b, handFaces(labels, colors, c.selected, c.cursor, s.Width))
	if v.Phase != 3 && s.PlayerID != v.TurnPlayer {
		fmt.Fprintf(&b, "\n%s等待 %s 操作%s\n", muted, playerName(s, v.TurnPlayer), reset)
	}
	fmt.Fprintln(&b, "操作：叫分 1/2/3 · 不叫 P · ←/→ 移动 · Space 选牌 · 1-9 快选 · Enter 出牌 · P 过牌 · Esc 清空选择")
	return b.String()
}

func (c *Landlord) Key(s ui.Snapshot, key string) ui.Result {
	var v landlordView
	if !decodeGame(s, &v) {
		return ui.Result{}
	}
	if c.selected == nil {
		c.Reset()
	}
	if key == " " {
		key = "space"
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "esc" || key == "escape" {
		c.Reset()
		return ui.Result{Handled: true, Status: "已清空选牌"}
	}
	if s.PlayerID != v.TurnPlayer {
		return ui.Result{}
	}
	if v.Phase == 0 {
		if key == "p" || key == "0" {
			return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "bid", "bid": 0}), Status: "不叫"}
		}
		if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= 3 {
			return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "bid", "bid": n}), Status: fmt.Sprintf("叫 %d 分", n)}
		}
		return ui.Result{}
	}
	if v.Phase != 1 {
		return ui.Result{}
	}
	if len(v.Hand) > 0 {
		switch key {
		case "left", "h":
			c.cursor = (c.cursor - 1 + len(v.Hand)) % len(v.Hand)
			return ui.Result{Handled: true}
		case "right", "l":
			c.cursor = (c.cursor + 1) % len(v.Hand)
			return ui.Result{Handled: true}
		case "home":
			c.cursor = 0
			return ui.Result{Handled: true}
		case "end":
			c.cursor = len(v.Hand) - 1
			return ui.Result{Handled: true}
		case "space":
			c.cursor = min(c.cursor, len(v.Hand)-1)
			key = strconv.Itoa(c.cursor + 1)
		}
	}
	if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(v.Hand) {
		if c.selected[n-1] {
			delete(c.selected, n-1)
		} else {
			c.selected[n-1] = true
		}
		return ui.Result{Handled: true, Status: fmt.Sprintf("已选择 %d 张", len(c.selected))}
	}
	if key == "p" {
		c.selected = map[int]bool{}
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "play", "cards": []gamecards.Card{}}), Status: "过牌"}
	}
	if key == "enter" || key == "return" {
		chosen := make([]gamecards.Card, 0, len(c.selected))
		for i, card := range v.Hand {
			if c.selected[i] {
				chosen = append(chosen, card)
			}
		}
		if len(chosen) == 0 {
			return ui.Result{Handled: true, Status: "先选择要出的牌；按 P 过牌"}
		}
		c.Reset()
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "play", "cards": chosen}), Status: fmt.Sprintf("出牌 %d 张", len(chosen))}
	}
	return ui.Result{}
}

func seatPlayerID(s ui.Snapshot, seat int) string {
	if seat >= 0 && seat < len(s.Room.Players) {
		return s.Room.Players[seat].ID
	}
	return ""
}
