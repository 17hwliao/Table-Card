package cards

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

type unoCard struct {
	ID     int   `json:"id"`
	Color  uint8 `json:"color"`
	Kind   uint8 `json:"kind"`
	Number uint8 `json:"number"`
}
type unoView struct {
	Players []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Cards   int    `json:"cards"`
		Score   int    `json:"score"`
		Bot     bool   `json:"bot"`
		SaidUNO bool   `json:"saidUNO"`
	} `json:"players"`
	Hand             []unoCard `json:"hand"`
	Turn             int       `json:"turn"`
	TurnPlayer       string    `json:"turnPlayer"`
	Discard          unoCard   `json:"discard"`
	Color            uint8     `json:"color"`
	Direction        int       `json:"direction"`
	DrawPenalty      int       `json:"drawPenalty"`
	PenaltyKind      uint8     `json:"penaltyKind"`
	Winner           string    `json:"winner"`
	SeriesWinner     string    `json:"seriesWinner"`
	Round            int       `json:"round"`
	Finished         bool      `json:"finished"`
	PendingChallenge bool      `json:"pendingChallenge"`
	ChallengeTarget  int       `json:"challengeTarget"`
	MissedUNO        int       `json:"missedUNO"`
	CanCatchUNO      bool      `json:"canCatchUNO"`
	DrewPlayable     bool      `json:"drewPlayable"`
}

type UNO struct {
	selected int
	color    uint8
	target   int
	announce bool
}

func NewUNO() *UNO            { return &UNO{selected: -1, target: -1} }
func (*UNO) Mode() table.Mode { return table.UNOMode }
func (c *UNO) Reset()         { c.selected = -1; c.color = 0; c.target = -1; c.announce = false }

func (c *UNO) View(s ui.Snapshot) string {
	var v unoView
	if !decodeGame(s, &v) {
		return "UNO 对局状态正在同步…"
	}
	var b strings.Builder
	fmt.Fprintln(&b, title(s, "UNO", fmt.Sprintf("第 %d 局   计分目标 500", v.Round)))
	fmt.Fprintln(&b)
	for i, p := range v.Players {
		label := p.Name
		if p.ID == s.PlayerID {
			label = "你 · " + label
		}
		if p.Bot {
			label += " [人机]"
		}
		marker := " "
		if p.ID == v.TurnPlayer && !v.Finished {
			marker = "▶"
		}
		uno := ""
		if p.SaidUNO {
			uno = " · UNO✓"
		}
		fmt.Fprintf(&b, "%s %-18s 手牌 %2d   积分 %3d%s\n", marker, label, p.Cards, p.Score, uno)
		_ = i
	}
	dir := "顺时针"
	if v.Direction < 0 {
		dir = "逆时针"
	}
	fmt.Fprintf(&b, "\n弃牌堆顶：%s     当前颜色：%s     方向：%s\n", unoName(v.Discard), unoColor(v.Color), dir)
	if v.DrawPenalty > 0 {
		fmt.Fprintf(&b, "%s累计罚牌 +%d：可叠加 +2 / +4，或按 D 承受%s\n", red, v.DrawPenalty, reset)
	}
	if v.PendingChallenge {
		fmt.Fprintf(&b, "%s万能 +4 待处理：按 X 挑战，或 D 接受%s\n", gold, reset)
	}
	if v.CanCatchUNO {
		fmt.Fprintf(&b, "%s有人漏喊 UNO：按 C 抓牌%s\n", red, reset)
	}
	if v.Finished {
		fmt.Fprintf(&b, "\n%s本局获胜：%s%s\n", gold, playerName(s, v.Winner), reset)
		if v.SeriesWinner != "" {
			fmt.Fprintf(&b, "整场获胜：%s\n", playerName(s, v.SeriesWinner))
		} else {
			fmt.Fprintln(&b, "按 N 开始下一局")
		}
	}
	fmt.Fprintf(&b, "\n%s你的手牌（数字选择）%s\n", gold, reset)
	if len(v.Hand) == 0 {
		fmt.Fprintln(&b, "（无手牌）")
	}
	for i, card := range v.Hand {
		mark := " "
		if i == c.selected {
			mark = gold + ">" + reset
		}
		fmt.Fprintf(&b, "%s%2d %s   ", mark, i+1, cardBox(unoName(card), i == c.selected, unoANSI(card.Color)))
		if (i+1)%6 == 0 {
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b)
	if c.color > 0 {
		fmt.Fprintf(&b, "\n已选万能牌颜色：%s\n", unoColor(c.color))
	}
	if c.announce {
		fmt.Fprintln(&b, "出牌时将同时喊出 UNO")
	}
	if c.target >= 0 && c.target < len(v.Players) {
		fmt.Fprintf(&b, "7 交换目标：%s（按 T 切换）\n", v.Players[c.target].Name)
	}
	if !v.Finished {
		if s.PlayerID == v.TurnPlayer {
			fmt.Fprintln(&b, "轮到你：数字选牌 · Enter 出牌 · D 摸牌/接受 · K 摸牌后结束 · U 切换喊 UNO · R/G/B/Y 选万能颜色")
			fmt.Fprintln(&b, "X 挑战 +4 · C 抓漏喊 · T 选择 7 的交换对象 · Esc 清空选择")
		} else {
			fmt.Fprintf(&b, "等待 %s 操作\n", playerName(s, v.TurnPlayer))
		}
	}
	return b.String()
}

func (c *UNO) Key(s ui.Snapshot, key string) ui.Result {
	var v unoView
	if !decodeGame(s, &v) {
		return ui.Result{}
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "esc" || key == "escape" {
		c.Reset()
		return ui.Result{Handled: true, Status: "已清空选择"}
	}
	if v.Finished {
		if key == "n" && v.SeriesWinner == "" {
			c.Reset()
			return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "next_round"}), Status: "开始下一局"}
		}
		return ui.Result{}
	}
	if s.PlayerID != v.TurnPlayer {
		return ui.Result{}
	}
	switch key {
	case "d":
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "draw"}), Status: "摸牌 / 接受罚牌"}
	case "k":
		if !v.DrewPlayable {
			return ui.Result{Handled: true, Status: "本回合尚未摸到牌"}
		}
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "keep"}), Status: "结束本回合"}
	case "x":
		if !v.PendingChallenge {
			return ui.Result{Handled: true, Status: "当前没有待挑战的万能 +4"}
		}
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "challenge"}), Status: "挑战万能 +4"}
	case "c":
		if !v.CanCatchUNO {
			return ui.Result{Handled: true, Status: "当前没有可抓漏喊 UNO 的玩家"}
		}
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "catch_uno"}), Status: "抓漏喊 UNO"}
	case "r":
		c.color = 1
		return ui.Result{Handled: true, Status: "选择红色"}
	case "y":
		c.color = 2
		return ui.Result{Handled: true, Status: "选择黄色"}
	case "b":
		c.color = 3
		return ui.Result{Handled: true, Status: "选择蓝色"}
	case "g":
		c.color = 4
		return ui.Result{Handled: true, Status: "选择绿色"}
	case "u":
		c.announce = !c.announce
		if c.announce {
			return ui.Result{Handled: true, Status: "本次出牌将喊 UNO"}
		}
		return ui.Result{Handled: true, Status: "取消喊 UNO"}
	case "t", "[", "]":
		if len(v.Players) < 2 {
			return ui.Result{Handled: true, Status: "没有可交换的对手"}
		}
		if c.target < 0 {
			for i, p := range v.Players {
				if p.ID != s.PlayerID {
					c.target = i
					break
				}
			}
		} else {
			step := 1
			if key == "[" {
				step = -1
			}
			c.target = (c.target + step + len(v.Players)) % len(v.Players)
			for v.Players[c.target].ID == s.PlayerID {
				c.target = (c.target + step + len(v.Players)) % len(v.Players)
			}
		}
		return ui.Result{Handled: true, Status: "7 的交换目标：" + v.Players[c.target].Name}
	case "enter", "return":
		if c.selected < 0 || c.selected >= len(v.Hand) {
			return ui.Result{Handled: true, Status: "先用数字选择一张手牌"}
		}
		card := v.Hand[c.selected]
		if (card.Kind == 4 || card.Kind == 5) && c.color == 0 {
			return ui.Result{Handled: true, Status: "出万能牌前请按 R/G/B/Y 选择颜色"}
		}
		target := c.target
		if card.Kind == 0 && card.Number == 7 && len(v.Hand) > 1 && target < 0 {
			return ui.Result{Handled: true, Status: "打出 7 前按 T 选择交换对象"}
		}
		if target < 0 {
			target = -1
		}
		action := map[string]any{"type": "play", "cardId": card.ID, "color": c.color, "uno": c.announce, "target": target}
		c.selected = -1
		c.color = 0
		c.announce = false
		return ui.Result{Handled: true, Action: ui.Action(action), Status: "打出 " + unoName(card)}
	}
	if key == "right" && len(v.Hand) > 0 {
		c.selected = (c.selected + 1) % len(v.Hand)
		return ui.Result{Handled: true, Status: fmt.Sprintf("选择第 %d 张牌", c.selected+1)}
	}
	if key == "left" && len(v.Hand) > 0 {
		c.selected--
		if c.selected < 0 {
			c.selected = len(v.Hand) - 1
		}
		return ui.Result{Handled: true, Status: fmt.Sprintf("选择第 %d 张牌", c.selected+1)}
	}
	if n, err := strconv.Atoi(key); err == nil {
		if n == 0 {
			n = 10
		}
		if n >= 1 && n <= len(v.Hand) {
			c.selected = n - 1
			return ui.Result{Handled: true, Status: fmt.Sprintf("选择第 %d 张：%s", n, unoName(v.Hand[n-1]))}
		}
	}
	return ui.Result{}
}

func unoName(c unoCard) string {
	label := ""
	switch c.Kind {
	case 1:
		label = "+2"
	case 2:
		label = "跳过"
	case 3:
		label = "反转"
	case 4:
		label = "万能"
	case 5:
		label = "万能+4"
	default:
		label = strconv.Itoa(int(c.Number))
	}
	if c.Kind == 4 || c.Kind == 5 {
		return label
	}
	return unoColor(c.Color) + " " + label
}
func unoColor(c uint8) string {
	switch c {
	case 1:
		return "红"
	case 2:
		return "黄"
	case 3:
		return "蓝"
	case 4:
		return "绿"
	default:
		return "未定"
	}
}
func unoANSI(c uint8) string {
	switch c {
	case 1:
		return "\x1b[31m"
	case 2:
		return "\x1b[33m"
	case 3:
		return "\x1b[34m"
	case 4:
		return "\x1b[32m"
	}
	return ""
}
