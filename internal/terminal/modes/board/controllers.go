// Package board contains keyboard-driven terminal controllers for the board games.
package board

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

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
	InCheck    bool   `json:"inCheck"`
	Winner     string `json:"winner"`
	Draw       bool   `json:"draw"`
	Moves      int    `json:"moves"`
	DrawReason string `json:"drawReason"`
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
	Scoring       bool          `json:"scoring"`
	Dead          [19][19]bool  `json:"dead"`
	ScoreAccepted [2]bool       `json:"scoreAccepted"`
	Board         [19][19]uint8 `json:"board"`
	Turn          uint8         `json:"turn"`
	TurnPlayer    string        `json:"turnPlayer"`
	LastMove      *struct {
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
	return gridView("五子棋 · 五子连珠", 15, 15, c.x, c.y, nil, func(x, y int) string {
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
	}, turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, g.Turn, "黑棋", "白棋"), "鼠标点击落子 · 方向键移动 · Enter 落子", "")
}
func (c *ChineseChess) View(s ui.Snapshot) string {
	var g chineseState
	if !decode(s.Game, &g) {
		return "正在同步中国象棋棋盘……"
	}
	turn := sideName(g.Turn, "红方", "黑方")
	status := turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, g.Turn, "红方", "黑方")
	if g.DrawReason != "" {
		status += " · " + g.DrawReason
	}
	if g.InCheck && g.Winner == "" {
		status += " · 将军"
	}
	if c.from != nil {
		status += fmt.Sprintf(" · 已选 %s", chinesePiece(g.Board[c.from[1]][c.from[0]].Kind, g.Board[c.from[1]][c.from[0]].Side))
	}
	return gridView("中国象棋 · 楚河汉界", 9, 10, c.x, c.y, c.from, func(x, y int) string {
		p := g.Board[y][x]
		if p.Kind == 0 {
			if y == 4 && x == 4 {
				return "╳"
			}
			return "·"
		}
		return pieceColor(chinesePiece(p.Kind, p.Side), p.Side, true)
	}, status, fmt.Sprintf("鼠标点击选子/走棋 · Enter 确认 · Esc 取消 · %s回合", turn), "")
}
func (c *InternationalChess) View(s ui.Snapshot) string {
	var g chessState
	if !decode(s.Game, &g) {
		return "正在同步国际象棋棋盘……"
	}
	status := turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, g.Turn, "白方", "黑方")
	if g.InCheck && g.Winner == "" {
		status += " · 将军"
	}
	if g.DrawReason != "" {
		status += " · " + g.DrawReason
	}
	if c.from != nil {
		status += fmt.Sprintf(" · 已选 %c%d", rune('a'+c.from[0]), 8-c.from[1])
	}
	return gridView("国际象棋 · Chess", 8, 8, c.x, c.y, c.from, func(x, y int) string {
		p := g.Board[y][x]
		if p.Kind == 0 {
			return "·"
		}
		return pieceColor(westernPiece(p.Kind, p.Side), p.Side, false)
	}, status, "鼠标点击选子/走棋 · Enter 确认 · Esc 取消 · 升变: Q/R/B/N", "")
}
func (c *Go) View(s ui.Snapshot) string {
	var g goState
	if !decode(s.Game, &g) {
		return "正在同步围棋棋盘……"
	}
	status := turnStatus(s.PlayerID, g.TurnPlayer, g.Winner, g.Draw, g.Moves, g.Turn, "黑棋", "白棋")
	if g.Finished {
		status += fmt.Sprintf(" · 数子 黑 %.1f : 白 %.1f", g.BlackScore, g.WhiteScore)
	}
	if g.Scoring {
		status = fmt.Sprintf("死子协商 · 黑 %.1f : 白 %.1f · 已确认 黑%t/白%t · Enter标死子 C确认 R继续", g.BlackScore, g.WhiteScore, g.ScoreAccepted[0], g.ScoreAccepted[1])
	}
	if g.LastMove != nil && g.LastMove.Pass {
		status += " · 上一手 Pass"
	}
	controls := "鼠标点击落子 · Enter 确认 · P 停一手"
	if g.Scoring {
		controls = "点击/Enter 切换整组死子 · C 确认结果 · R 恢复行棋"
	}
	return gridView("围棋 · 19 路", 19, 19, c.x, c.y, nil, func(x, y int) string {
		if g.Dead[y][x] {
			return "×"
		}
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
	}, status, controls, "")
}

func (c *Gomoku) Key(s ui.Snapshot, key string) ui.Result {
	var g gomokuState
	if !decode(s.Game, &g) {
		return ui.Result{Status: "正在同步棋盘"}
	}
	if done(g.Winner, g.Draw) {
		return ui.Result{Handled: true, Status: "本局已结束"}
	}
	moveCursor(&c.x, &c.y, 15, 15, key)
	if key == "enter" || key == " " {
		if g.TurnPlayer != s.PlayerID {
			return ui.Result{Handled: true, Status: "请等待你的回合"}
		}
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
	moveCursor(&c.x, &c.y, 9, 10, key)
	if key == "esc" || key == "escape" {
		c.from = nil
		return ui.Result{Handled: true, Status: "取消选子"}
	}
	if key != "enter" && key != " " {
		return handledMove(key)
	}
	if g.TurnPlayer != s.PlayerID {
		return ui.Result{Handled: true, Status: "请等待你的回合"}
	}
	if c.from != nil && g.Board[c.y][c.x].Side == g.Turn {
		c.from = &[2]int{c.x, c.y}
		return ui.Result{Handled: true, Status: "已切换选中的棋子"}
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
	moveCursor(&c.x, &c.y, 8, 8, key)
	if key == "esc" || key == "escape" {
		c.from = nil
		return ui.Result{Handled: true, Status: "取消选子"}
	}
	if key != "enter" && key != " " {
		return handledMove(key)
	}
	if g.TurnPlayer != s.PlayerID {
		return ui.Result{Handled: true, Status: "请等待你的回合"}
	}
	if c.from != nil && g.Board[c.y][c.x].Side == g.Turn {
		c.from = &[2]int{c.x, c.y}
		return ui.Result{Handled: true, Status: "已切换选中的棋子"}
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
	moveCursor(&c.x, &c.y, 19, 19, key)
	if g.Scoring {
		switch strings.ToLower(key) {
		case "c":
			return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "accept_score"})}
		case "r":
			return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "resume"})}
		case "enter", " ":
			return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "mark_dead", "x": c.x, "y": c.y})}
		case "p":
			return ui.Result{Handled: true, Status: "请按 C 确认数子，或 R 恢复行棋"}
		}
		return handledMove(key)
	}
	if strings.EqualFold(key, "p") {
		return ui.Result{Handled: true, Action: ui.Action(map[string]any{"type": "pass"})}
	}
	if key == "enter" || key == " " {
		if g.TurnPlayer != s.PlayerID {
			return ui.Result{Handled: true, Status: "请等待你的回合"}
		}
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
func moveCursor(x, y *int, width, height int, key string) {
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
	if *x >= width {
		*x = width - 1
	}
	if *y >= height {
		*y = height - 1
	}
}
func turnStatus(player, turn, winner string, draw bool, moves int, color uint8, first, second string) string {
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
	if color == 1 {
		side = first
	}
	if turn == player {
		return fmt.Sprintf("第 %d 手 · %s走棋 · 轮到你", moves+1, side)
	}
	return fmt.Sprintf("第 %d 手 · %s走棋 · 等待对手", moves+1, side)
}
func sideName(side uint8, a, b string) string {
	if side == 1 {
		return a
	}
	return b
}

// Board cells begin at column 4, row 2 in View. Mouse uses this same geometry.
func gridView(title string, cols, rows, cx, cy int, selected *[2]int, cell func(int, int) string, status, help, extra string) string {
	cw := cellWidth(cols)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5CF82")).Render("  "+title) + "\n")
	b.WriteString("    ")
	for x := 0; x < cols; x++ {
		label := fmt.Sprint(x + 1)
		if cols == 8 {
			label = string(rune('a' + x))
		}
		b.WriteString(center(label, cw))
	}
	b.WriteByte('\n')
	for y := 0; y < rows; y++ {
		fmt.Fprintf(&b, "%2d │", rows-y)
		for x := 0; x < cols; x++ {
			variant := 0
			if cols == 8 && (x+y)%2 == 0 {
				variant = 1
			}
			if selected != nil && selected[0] == x && selected[1] == y {
				variant = 2
			}
			if x == cx && y == cy {
				variant = 3
			}
			b.WriteString(renderCell(cell(x, y), cw, variant))
		}
		b.WriteString("│\n")
	}
	b.WriteString("   ╰" + strings.Repeat("─", cols*cw) + "╯\n")
	b.WriteString(status + "\n" + help)
	if extra != "" {
		b.WriteString("\n" + extra)
	}
	return b.String()
}

type cellKey struct {
	text           string
	width, variant int
}

var cellCache = struct {
	sync.Mutex
	entries map[cellKey]string
}{entries: make(map[cellKey]string)}
var cellStyles = [4]lipgloss.Style{
	lipgloss.NewStyle().Background(lipgloss.Color("#443A2D")),
	lipgloss.NewStyle().Background(lipgloss.Color("#6A6352")),
	lipgloss.NewStyle().Background(lipgloss.Color("#246651")).Bold(true),
	lipgloss.NewStyle().Background(lipgloss.Color("#52648A")).Bold(true),
}

// Cache only a bounded vocabulary of cells, never entire boards or matches.
func renderCell(text string, width, variant int) string {
	key := cellKey{text, width, variant}
	cellCache.Lock()
	cached, ok := cellCache.entries[key]
	cellCache.Unlock()
	if ok {
		return cached
	}
	rendered := cellStyles[variant].Render(center(text, width))
	cellCache.Lock()
	if len(cellCache.entries) < 512 {
		cellCache.entries[key] = rendered
	}
	cellCache.Unlock()
	return rendered
}
func cellWidth(cols int) int {
	if cols == 8 || cols == 9 {
		return 4
	}
	return 3
}
func center(s string, w int) string {
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}
func pieceColor(s string, side uint8, chinese bool) string {
	color := "#E9EDF5"
	if side == 2 {
		color = "#A6C9E8"
	} else if chinese {
		color = "#FF8D85"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(s)
}
func hitCell(x, y, cols, rows int) (int, int, bool) {
	x -= 4
	y -= 2
	if x < 0 || y < 0 || x >= cols*cellWidth(cols) || y >= rows {
		return 0, 0, false
	}
	return x / cellWidth(cols), y, true
}
func (c *Gomoku) Mouse(s ui.Snapshot, x, y int) ui.Result {
	bx, by, ok := hitCell(x, y, 15, 15)
	if !ok {
		return ui.Result{}
	}
	c.x, c.y = bx, by
	return c.Key(s, "enter")
}
func (c *ChineseChess) Mouse(s ui.Snapshot, x, y int) ui.Result {
	bx, by, ok := hitCell(x, y, 9, 10)
	if !ok {
		return ui.Result{}
	}
	c.x, c.y = bx, by
	return c.Key(s, "enter")
}
func (c *InternationalChess) Mouse(s ui.Snapshot, x, y int) ui.Result {
	bx, by, ok := hitCell(x, y, 8, 8)
	if !ok {
		return ui.Result{}
	}
	c.x, c.y = bx, by
	return c.Key(s, "enter")
}
func (c *Go) Mouse(s ui.Snapshot, x, y int) ui.Result {
	bx, by, ok := hitCell(x, y, 19, 19)
	if !ok {
		return ui.Result{}
	}
	c.x, c.y = bx, by
	return c.Key(s, "enter")
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
