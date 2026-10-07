// Package tetris renders a keyboard-controlled terminal arcade table.
package tetris

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
	game "github.com/17hwliao/table-card-independent/internal/tetris"
	"github.com/charmbracelet/x/ansi"
)

type Controller struct{}

func New() *Controller               { return &Controller{} }
func (*Controller) Mode() table.Mode { return table.TetrisMode }
func (*Controller) Reset()           {}

var colors = [...]string{"#182723", "#63D8E5", "#F3D35B", "#BE8BF5", "#76D68C", "#F17783", "#719EF2", "#F5AC62"}
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#77968B"))
var highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("#E9B44C")).Bold(true)

// Reuse cell strings across realtime frames instead of rebuilding hundreds of
// style objects on every falling-block update.
var solid, moving [8]string
var halves [8][8]string
var emptyCell, ghostCell, edge string
var pieceNames = [...]string{"", "I", "O", "T", "S", "Z", "J", "L"}

func init() {
	emptyCell, ghostCell, edge = muted.Render("· "), muted.Render("░░"), muted.Render("│")
	for i := range colors {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[i]))
		solid[i], moving[i] = style.Render("██"), style.Render("▓▓")
		for j := range colors {
			halves[i][j] = style.Background(lipgloss.Color(colors[j])).Render("▀")
		}
	}
}

func (*Controller) Key(s ui.Snapshot, key string) ui.Result {
	action := ""
	switch key {
	case "left", "a":
		action = "left"
	case "right", "d":
		action = "right"
	case "up", "w":
		action = "rotate_cw"
	case "down", "z":
		action = "rotate_ccw"
	case "s":
		action = "soft_drop"
	case "space", " ":
		action = "hard_drop"
	default:
		return ui.Result{}
	}
	var v game.Snapshot
	if json.Unmarshal(s.Game, &v) != nil {
		return ui.Result{Handled: true}
	}
	if v.Finished {
		return ui.Result{Handled: true, Status: "比赛结束 · F2 再来一局 · Del 回到模式大厅"}
	}
	if !v.Started {
		return ui.Result{Handled: true, Status: "开局倒计时中，请稍候"}
	}
	for _, p := range v.Players {
		if p.ID == s.PlayerID && !p.Alive {
			return ui.Result{Handled: true, Status: "你已淘汰，正在观看比赛 · Del 离桌"}
		}
	}
	return ui.Result{Handled: true, Action: ui.Action(map[string]string{"type": action})}
}

func (*Controller) View(s ui.Snapshot) string {
	var v game.Snapshot
	if json.Unmarshal(s.Game, &v) != nil || len(v.Players) == 0 {
		return "正在铺开方块棋盘…"
	}
	mine := 0
	for i, p := range v.Players {
		if p.ID == s.PlayerID {
			mine = i
			break
		}
	}
	p := v.Players[mine]
	status := fmt.Sprintf("Lv%d · %dms/格 · 已过 %ds · 堆到顶部淘汰", max(1, v.Level), v.DropMS, v.ElapsedSeconds)
	if v.NextLevelIn > 0 {
		status += fmt.Sprintf(" · %ds后加速", v.NextLevelIn)
	}
	if !v.Started {
		remaining := max(int64(1), (v.StartAt-time.Now().UnixMilli()+999)/1000)
		status = fmt.Sprintf("准备 %d 秒 · 所有玩家使用相同方块序列", remaining)
	}
	if !p.Alive {
		status = fmt.Sprintf("你已淘汰 · 第 %d 名 · 观看剩余玩家", p.Place)
	}
	if v.Finished {
		if v.Draw {
			status = "比赛结束 · 同时顶满，平局"
		} else {
			for _, winner := range v.Players {
				if winner.ID == v.Winner {
					status = "比赛结束 · " + winner.Name + " 获胜"
				}
			}
		}
	}
	// Keep the local board full size. Opponents use half-height cells on
	// smaller terminals, so all four players remain visible beside each other.
	var tableView string
	if s.Height > 0 && s.Height < 30 {
		boards := []string{miniBoard(p, true)}
		for i, opponent := range v.Players {
			if i != mine {
				boards = append(boards, miniBoard(opponent, false))
			}
		}
		tableView = joinBoards(boards, " ") + "\n" + muted.Render("你的下一块："+pieceNames[p.Next])
	} else if s.Width >= len(v.Players)*23 {
		boards := []string{fullBoard(p, true)}
		for i, opponent := range v.Players {
			if i != mine {
				boards = append(boards, fullBoard(opponent, false))
			}
		}
		tableView = joinBoards(boards, " ")
	} else {
		previews := []string{}
		for i, opponent := range v.Players {
			if i != mine {
				previews = append(previews, miniBoard(opponent, false))
			}
		}
		right := joinBoards(previews, " ") + "\n\n" + highlight.Render("下一块") + "\n" + nextPreview(p.Next) + "\n" + muted.Render("↑ 顺时针 / ↓ 逆时针\n← → 移动 · S 加速\n空格 落底 · F1 规则")
		if s.Width < 22+2+(len(v.Players)-1)*13 {
			var rows []string
			for i, opponent := range v.Players {
				if i != mine {
					rows = append(rows, playerLabel(opponent, false)+fmt.Sprintf(" · %d 行", opponent.Lines))
				}
			}
			right = strings.Join(rows, "\n") + "\n\n下一块\n" + nextPreview(p.Next)
		}
		tableView = ui.JoinTop(fullBoard(p, true), "  ", right)
	}
	return highlight.Render(status) + "\n" + tableView + "\n" + muted.Render("←/→ 移动 · ↑/↓ 旋转 · S 加速 · 空格 落底")
}

func joinBoards(boards []string, separator string) string {
	var parts []string
	for i, b := range boards {
		if i > 0 {
			parts = append(parts, separator)
		}
		parts = append(parts, b)
	}
	return ui.JoinTop(parts...)
}
func playerLabel(p game.Player, mine bool) string {
	name := p.Name
	if mine {
		name = "你 · " + name
	}
	if !p.Alive {
		name = fmt.Sprintf("#%d 淘汰 · %s", p.Place, p.Name)
	}
	return name
}
func displayed(p game.Player, ghost bool) (game.Board, [game.Height][game.Width]bool) {
	board := p.Board
	active := [game.Height][game.Width]bool{}
	if !p.Alive {
		return board, active
	}
	if ghost {
		for _, c := range game.Cells(game.Ghost(p.Board, p.Active)) {
			if c.Y >= 0 && c.Y < game.Height && c.X >= 0 && c.X < game.Width && board[c.Y][c.X] == 0 {
				board[c.Y][c.X] = 8
			}
		}
	}
	for _, c := range game.Cells(p.Active) {
		if c.Y >= 0 && c.Y < game.Height && c.X >= 0 && c.X < game.Width {
			board[c.Y][c.X] = uint8(p.Active.Kind)
			active[c.Y][c.X] = true
		}
	}
	return board, active
}
func fullBoard(p game.Player, mine bool) string {
	board, active := displayed(p, mine)
	var b strings.Builder
	label := ansi.Truncate(playerLabel(p, mine), 22, "…")
	b.WriteString(highlight.Render(label) + "\n")
	b.WriteString(muted.Render("┌────────────────────┐") + "\n")
	for y := 0; y < game.Height; y++ {
		b.WriteString(edge)
		for x := 0; x < game.Width; x++ {
			value := board[y][x]
			switch {
			case value == 0:
				b.WriteString(emptyCell)
			case value == 8:
				b.WriteString(ghostCell)
			default:
				if active[y][x] {
					b.WriteString(moving[value])
				} else {
					b.WriteString(solid[value])
				}
			}
		}
		b.WriteString(edge + "\n")
	}
	b.WriteString(muted.Render("└────────────────────┘") + "\n")
	footer := fmt.Sprintf("%d分 · %d行 · 下%s", p.Score, p.Lines, pieceNames[p.Next])
	b.WriteString(ansi.Truncate(footer, 22, "…"))
	return b.String()
}
func miniBoard(p game.Player, mine bool) string {
	board, _ := displayed(p, false)
	var b strings.Builder
	b.WriteString(highlight.Render(ansi.Truncate(playerLabel(p, mine), 12, "…")) + "\n")
	b.WriteString(muted.Render("┌──────────┐") + "\n")
	for y := 0; y < game.Height; y += 2 {
		b.WriteString(edge)
		for x := 0; x < game.Width; x++ {
			top, bottom := board[y][x], board[y+1][x]
			b.WriteString(halves[top][bottom])
		}
		b.WriteString(edge + "\n")
	}
	b.WriteString(muted.Render("└──────────┘") + "\n")
	b.WriteString(ansi.Truncate(fmt.Sprintf("%d行 · %d分", p.Lines, p.Score), 12, "…"))
	return b.String()
}
func nextPreview(kind int) string {
	cells := game.Cells(game.Piece{Kind: kind})
	var b strings.Builder
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			found := false
			for _, c := range cells {
				if c.X == x && c.Y == y {
					found = true
					break
				}
			}
			if found {
				b.WriteString(solid[kind])
			} else {
				b.WriteString("  ")
			}
		}
		if y == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
