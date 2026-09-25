// Package board contains keyboard-driven terminal controllers for the board games.
package board

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

type gomokuState struct {
	Board      [15][15]uint8 `json:"board"`
	Turn       uint8         `json:"turn"`
	TurnPlayer string        `json:"turnPlayer"`
	LastMove   *struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"lastMove"`
	Winner string `json:"winner"`
	Draw   bool   `json:"draw"`
	Moves  int    `json:"moves"`
}
type chineseState struct {
	Board [10][9]struct {
		Side uint8 `json:"side"`
		Kind uint8 `json:"kind"`
	} `json:"board"`
	Turn       uint8  `json:"turn"`
	TurnPlayer string `json:"turnPlayer"`
	LastMove   *struct {
		From struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"from"`
		To struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"to"`
	} `json:"lastMove"`
	InCheck bool   `json:"inCheck"`
	Winner  string `json:"winner"`
	Draw    bool   `json:"draw"`
	Moves   int    `json:"moves"`
}
type chessState struct {
	Board [8][8]struct {
		Side uint8 `json:"side"`
		Kind uint8 `json:"kind"`
	} `json:"board"`
	Turn       uint8  `json:"turn"`
	TurnPlayer string `json:"turnPlayer"`
	LastMove   *struct {
		From struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"from"`
		To struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"to"`
		Promotion uint8 `json:"promotion"`
	} `json:"lastMove"`
	InCheck    bool   `json:"inCheck"`
	Winner     string `json:"winner"`
	Draw       bool   `json:"draw"`
	Moves      int    `json:"moves"`
	DrawReason string `json:"drawReason"`
}
type goState struct {
	Board      [19][19]uint8 `json:"board"`
	Turn       uint8         `json:"turn"`
	TurnPlayer string        `json:"turnPlayer"`
	LastMove   *struct {
		X    int  `json:"x"`
		Y    int  `json:"y"`
		Pass bool `json:"pass"`
	} `json:"lastMove"`
	Winner            string  `json:"winner"`
	Draw              bool    `json:"draw"`
	Finished          bool    `json:"finished"`
	Moves             int     `json:"moves"`
	ConsecutivePasses int     `json:"consecutivePasses"`
	Captures          [3]int  `json:"captures"`
	BlackScore        float64 `json:"blackScore"`
	WhiteScore        float64 `json:"whiteScore"`
}

type Gomoku struct{ x, y int }
type ChineseChess struct {
	x, y int
	from *[2]int
}
type InternationalChess struct {
	x, y      int
	from      *[2]int
	promotion uint8
}
type Go struct{ x, y int }

func (*Gomoku) Mode() table.Mode             { return table.GomokuMode }
func (*ChineseChess) Mode() table.Mode       { return table.ChessMode }
func (*InternationalChess) Mode() table.Mode { return table.WesternChessMode }
func (*Go) Mode() table.Mode                 { return table.GoMode }
func (c *Gomoku) Reset()                     { c.x, c.y = 7, 7 }
func (c *ChineseChess) Reset()               { c.x, c.y = 4, 9; c.from = nil }
func (c *InternationalChess) Reset()         { c.x, c.y = 4, 7; c.from = nil; c.promotion = 5 }
func (c *Go) Reset()                         { c.x, c.y = 9, 9 }

func (c *Gomoku) View(s ui.Snapshot) string {
	var g gomokuState
	if !decode(s.Game, &g) {
		return "正在同步五子棋棋盘……"
	}
	return gridView("五子棋 · 五子连珠", 15, c.x, c.y, func(x, y int) string {
		switch g.Board[y][x] {
		case 1:
			return "●"
		case 2:
			return "○"
		default:
			if x == 7 && y == 7 {
				return "┼"
			}
			return "·"
		}
	}, turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, "黑棋", "白棋"), "方向键移动 · Enter 落子 · X/Y 选择坐标", "")
}
func (c *ChineseChess) View(s ui.Snapshot) string {
	var g chineseState
	if !decode(s.Game, &g) {
		return "正在同步中国象棋棋盘……"
	}
	turn := sideName(g.Turn, "红方", "黑方")
	status := turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, "红方", "黑方")
	if g.InCheck && g.Winner == "" {
		status += " · 将军"
	}
	if c.from != nil {
		status += fmt.Sprintf(" · 已选 %s", chinesePiece(g.Board[c.from[1]][c.from[0]].Kind, g.Board[c.from[1]][c.from[0]].Side))
	}
	return gridView("中国象棋 · 楚河汉界", 9, c.x, c.y, func(x, y int) string {
		p := g.Board[y][x]
		if p.Kind == 0 {
			if y == 4 && x == 4 {
				return "╳"
			}
			return "·"
		}
		return chinesePiece(p.Kind, p.Side)
	}, status, fmt.Sprintf("方向键移动 · Enter 选子/走棋 · %s回合", turn), "")
}
func (c *InternationalChess) View(s ui.Snapshot) string {
	var g chessState
	if !decode(s.Game, &g) {
		return "正在同步国际象棋棋盘……"
	}
	status := turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, "白方", "黑方")
	if g.InCheck && g.Winner == "" {
		status += " · 将军"
	}
	if g.DrawReason != "" {
		status += " · " + g.DrawReason
	}
	if c.from != nil {
		status += fmt.Sprintf(" · 已选 %c%d", rune('a'+c.from[0]), 8-c.from[1])
	}
	return gridView("国际象棋 · Chess", 8, c.x, c.y, func(x, y int) string {
		p := g.Board[y][x]
		if p.Kind == 0 {
			return "·"
		}
		return westernPiece(p.Kind, p.Side)
	}, status, "方向键移动 · Enter 选子/走棋 · 升变: Q/R/B/N", "")
}
func (c *Go) View(s ui.Snapshot) string {
	var g goState
	if !decode(s.Game, &g) {
		return "正在同步围棋棋盘……"
	}
	status := turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, "黑棋", "白棋")
	if g.Finished {
		status += fmt.Sprintf(" · 数子 黑 %.1f : 白 %.1f", g.BlackScore, g.WhiteScore)
	}
	if g.LastMove != nil && g.LastMove.Pass {
		status += " · 上一手 Pass"
	}
	return gridView("围棋 · 19 路", 19, c.x, c.y, func(x, y int) string {
		switch g.Board[y][x] {
		case 1:
			return "●"
		case 2:
			return "○"
		default:
			if x == 3 && y == 3 || x == 9 && y == 3 || x == 15 && y == 3 || x == 3 && y == 9 || x == 9 && y == 9 || x == 15 && y == 9 || x == 3 && y == 15 || x == 9 && y == 15 || x == 15 && y == 15 {
				return "┼"
			}
			return "+"
		}
	}, status, "方向键移动 · Enter 落子 · P 停一手 Pass", "")
}

func (c *Gomoku) Key(s ui.Snapshot, key string) ui.Result {
	var g gomokuState
	if !decode(s.Game, &g) {
		return ui.Result{Status: "正在同步棋盘"}
	}
	if done(g.Winner, g.Draw) {
		return ui.Result{Handled: true, Status: "本局已结束"}
	}
	moveCursor(&c.x, &c.y, 15, key)
	if key == "enter" || key == " " {
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "place", "x": c.x, "y": c.y})}
	}
	return handledMove(key)
}
func (c *ChineseChess) Key(s ui.Snapshot, key string) ui.Result {
	var g chineseState
	if !decode(s.Game, &g) {
		return ui.Result{Status: "正在同步棋盘"}
	}
	if done(g.Winner, g.Draw) {
		return ui.Result{Handled: true, Status: "本局已结束"}
	}
	moveCursor(&c.x, &c.y, 9, key)
	if key == "esc" || key == "escape" {
		c.from = nil
		return ui.Result{Handled: true, Status: "取消选子"}
	}
	if key != "enter" && key != " " {
		return handledMove(key)
	}
	if c.from == nil {
		if g.Board[c.y][c.x].Side != g.Turn {
			return ui.Result{Handled: true, Status: "请选择当前回合的己方棋子"}
		}
		c.from = &[2]int{c.x, c.y}
		return ui.Result{Handled: true, Status: "已选棋子，请移动到目标点"}
	}
	from := *c.from
	c.from = nil
	return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "move", "fromX": from[0], "fromY": from[1], "toX": c.x, "toY": c.y})}
}
func (c *InternationalChess) Key(s ui.Snapshot, key string) ui.Result {
	var g chessState
	if !decode(s.Game, &g) {
		return ui.Result{Status: "正在同步棋盘"}
	}
	if done(g.Winner, g.Draw) {
		return ui.Result{Handled: true, Status: "本局已结束"}
	}
	if strings.EqualFold(key, "q") {
		c.promotion = 5
		return ui.Result{Handled: true, Status: "兵升变：后"}
	}
	if strings.EqualFold(key, "r") {
		c.promotion = 4
		return ui.Result{Handled: true, Status: "兵升变：车"}
	}
	if strings.EqualFold(key, "b") {
		c.promotion = 3
		return ui.Result{Handled: true, Status: "兵升变：象"}
	}
	if strings.EqualFold(key, "n") {
		c.promotion = 2
		return ui.Result{Handled: true, Status: "兵升变：马"}
	}
	moveCursor(&c.x, &c.y, 8, key)
	if key == "esc" || key == "escape" {
		c.from = nil
		return ui.Result{Handled: true, Status: "取消选子"}
	}
	if key != "enter" && key != " " {
		return handledMove(key)
	}
	if c.from == nil {
		if g.Board[c.y][c.x].Side != g.Turn {
			return ui.Result{Handled: true, Status: "请选择当前回合的己方棋子"}
		}
		c.from = &[2]int{c.x, c.y}
		return ui.Result{Handled: true, Status: "已选棋子，请选择目标格"}
	}
	from := *c.from
	c.from = nil
	return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "move", "fromX": from[0], "fromY": from[1], "toX": c.x, "toY": c.y, "promotion": c.promotion})}
}
func (c *Go) Key(s ui.Snapshot, key string) ui.Result {
	var g goState
	if !decode(s.Game, &g) {
		return ui.Result{Status: "正在同步棋盘"}
	}
	if g.Finished {
		return ui.Result{Handled: true, Status: "本局已结束"}
	}
	moveCursor(&c.x, &c.y, 19, key)
	if strings.EqualFold(key, "p") {
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "pass"})}
	}
	if key == "enter" || key == " " {
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "play", "x": c.x, "y": c.y})}
	}
	return handledMove(key)
}

func decode(raw json.RawMessage, v any) bool { return len(raw) > 0 && json.Unmarshal(raw, v) == nil }
func done(winner string, draw bool) bool     { return winner != "" || draw }
func handledMove(key string) ui.Result {
	switch strings.ToLower(key) {
	case "up", "k", "w", "↑", "down", "j", "s", "↓", "left", "h", "a", "←", "right", "l", "d", "→":
		return ui.Result{Handled: true}
	}
	return ui.Result{}
}
func moveCursor(x, y *int, size int, key string) {
	switch strings.ToLower(key) {
	case "up", "k", "w", "↑":
		*y--
	case "down", "j", "s", "↓":
		*y++
	case "left", "h", "a", "←":
		*x--
	case "right", "l", "d", "→":
		*x++
	}
	if *x < 0 {
		*x = 0
	}
	if *y < 0 {
		*y = 0
	}
	if *x >= size {
		*x = size - 1
	}
	if *y >= size {
		*y = size - 1
	}
}
func turnStatus(player, turn, winner string, draw bool, moves int, first, second string) string {
	if winner != "" {
		if winner == player {
			return fmt.Sprintf("你获胜 · 共 %d 手", moves)
		}
		return fmt.Sprintf("对局结束 · 胜者 %s", winner)
	}
	if draw {
		return fmt.Sprintf("和棋 · 共 %d 手", moves)
	}
	side := second
	if turn != "" && moves%2 == 0 {
		side = first
	}
	return fmt.Sprintf("第 %d 手 · %s走棋", moves+1, side)
}
func sideName(side uint8, a, b string) string {
	if side == 1 {
		return a
	}
	return b
}
func gridView(title string, size, cx, cy int, cell func(int, int) string, status, help, extra string) string {
	cellw := 2
	if size == 8 {
		cellw = 4
	}
	if size == 9 {
		cellw = 4
	}
	var b strings.Builder
	b.WriteString("╭─ " + title + " " + strings.Repeat("─", maxInt(2, size*cellw-len([]rune(title)))) + "╮\n")
	if size == 8 {
		b.WriteString("    a   b   c   d   e   f   g   h\n")
	}
	for y := 0; y < size; y++ {
		if size == 8 {
			fmt.Fprintf(&b, " %d │", 8-y)
		} else if size == 9 {
			fmt.Fprintf(&b, "%2d │", size-y)
		} else if size == 19 {
			fmt.Fprintf(&b, "%2d │", size-y)
		} else {
			b.WriteString("   │")
		}
		for x := 0; x < size; x++ {
			v := cell(x, y)
			if x == cx && y == cy {
				v = "[" + v + "]"
			} else if size == 15 || size == 19 {
				v = " " + v
			}
			b.WriteString(center(v, cellw))
		}
		b.WriteString("│\n")
	}
	b.WriteString("   ╰" + strings.Repeat("─", size*cellw) + "╯\n")
	if size != 8 {
		b.WriteString("     ")
		for x := 0; x < size; x++ {
			fmt.Fprintf(&b, "%*d", cellw, x+1)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n" + status + "\n" + help)
	if extra != "" {
		b.WriteString("\n" + extra)
	}
	return b.String()
}
func center(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func chinesePiece(kind, side uint8) string {
	red := []string{"", "帥", "仕", "相", "馬", "車", "炮", "兵"}
	black := []string{"", "將", "士", "象", "馬", "車", "砲", "卒"}
	if kind >= uint8(len(red)) {
		return "?"
	}
	if side == 1 {
		return red[kind]
	}
	return black[kind]
}
func westernPiece(kind, side uint8) string {
	black := []string{"", "♟", "♞", "♝", "♜", "♛", "♚"}
	white := []string{"", "♙", "♘", "♗", "♖", "♕", "♔"}
	if kind >= uint8(len(black)) {
		return "?"
	}
	if side == 1 {
		return white[kind]
	}
	return black[kind]
}
