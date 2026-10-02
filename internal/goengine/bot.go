package goengine

import (
	"encoding/json"
	"errors"
)

// candidate is shared by human moves and bot planning so suicide and superko
// cannot differ between the two paths. Caller must hold the game lock.
func (g *Game) candidate(x, y int) ([BoardSize][BoardSize]uint8, int, error) {
	next := g.board
	if !inBoard(x, y) || next[y][x] != Empty {
		return next, 0, errors.New("落子位置无效或已占用")
	}
	next[y][x] = g.turn
	captured := 0
	seen := [BoardSize][BoardSize]bool{}
	var scratch [BoardSize * BoardSize][2]int
	for _, d := range neighbors(x, y) {
		nx, ny := x+d[0], y+d[1]
		if !inBoard(nx, ny) || next[ny][nx] != opponent(g.turn) || seen[ny][nx] {
			continue
		}
		group, liberties := collectInto(next, nx, ny, &scratch)
		for _, p := range group {
			seen[p[1]][p[0]] = true
		}
		if liberties == 0 {
			for _, p := range group {
				next[p[1]][p[0]] = Empty
				captured++
			}
		}
	}
	if _, liberties := collectInto(next, x, y, &scratch); liberties == 0 {
		return next, 0, errors.New("禁止自杀着")
	}
	if g.history[next] {
		return next, 0, errors.New("全局同形禁止：落子不能还原本局曾经出现的棋形")
	}
	return next, captured, nil
}

func (e *Engine) BotAction(playerID string) json.RawMessage {
	g := e.game
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.finished || g.players[int(g.turn)-1].ID != playerID {
		return nil
	}
	best, bx, by := -1<<30, -1, -1
	var scratch [BoardSize * BoardSize][2]int
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			next, captured, err := g.candidate(x, y)
			if err != nil {
				continue
			}
			_, libs := collectInto(next, x, y, &scratch)
			own, enemy, border := 0, 0, 0
			rescue := 0
			for _, d := range neighbors(x, y) {
				nx, ny := x+d[0], y+d[1]
				if !inBoard(nx, ny) {
					border++
					continue
				}
				if g.board[ny][nx] == g.turn {
					own++
					group, before := collectInto(g.board, nx, ny, &scratch)
					if before == 1 && libs > 1 {
						rescue += len(group) * 12
					}
				} else if g.board[ny][nx] == opponent(g.turn) {
					enemy++
				}
			}
			// Preserve eyes instead of filling already enclosed friendly territory.
			if own+border == 4 && captured == 0 {
				continue
			}
			value := captured*100 + rescue + min(libs, 4)*3 + enemy*4 - own*3
			if libs == 1 {
				value -= 90
			}
			edge := min(x, y, BoardSize-1-x, BoardSize-1-y)
			if edge < 2 {
				value -= (2 - edge) * 4
			}
			if g.moves < 12 {
				value -= abs(edge-3) * 2
			}
			if value > best {
				best, bx, by = value, x, y
			}
		}
	}
	if bx < 0 || best < 0 || (g.passes > 0 && g.moves > 100 && best < 20) {
		a, _ := json.Marshal(map[string]any{"type": "pass"})
		return a
	}
	a, _ := json.Marshal(map[string]any{"type": "play", "x": bx, "y": by})
	return a
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
