package chinesechess

import "encoding/json"

// BotAction only emits legal moves and prefers captures, checks and safe development.
func (e *Engine) BotAction(playerID string) json.RawMessage {
	g := e.game
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.winner != "" || g.draw || g.players[int(g.turn)-1].ID != playerID {
		return nil
	}
	values := [8]int{0, 10000, 200, 200, 400, 900, 450, 100}
	best := -1 << 30
	var chosen *Move
	other := Red
	if g.turn == Red {
		other = Black
	}
	for y := 0; y < Ranks; y++ {
		for x := 0; x < Files; x++ {
			piece := g.board[y][x]
			if piece.Side != g.turn {
				continue
			}
			from := Point{x, y}
			for ty := 0; ty < Ranks; ty++ {
				for tx := 0; tx < Files; tx++ {
					to := Point{tx, ty}
					if !pieceCanMove(g.board, from, to) || g.board[ty][tx].Kind == General {
						continue
					}
					next := g.board
					next[ty][tx], next[y][x] = piece, Piece{}
					if inCheck(next, g.turn) {
						continue
					}
					value := values[g.board[ty][tx].Kind]*10 + (4-abs(tx-4))*3
					if piece.Kind == Soldier {
						if g.turn == Red {
							value += (9 - ty) * 6
						} else {
							value += ty * 6
						}
					}
					if inCheck(next, other) {
						value += 35
						if !hasLegalMove(next, other) {
							value += 1000000
						}
					}
					attacked := false
					for ey := 0; ey < Ranks && !attacked; ey++ {
						for ex := 0; ex < Files; ex++ {
							if next[ey][ex].Side == other && pieceCanMove(next, Point{ex, ey}, to) {
								attacked = true
								break
							}
						}
					}
					if attacked {
						value -= values[piece.Kind] * 9
					}
					if g.last != nil && g.last.From == to {
						value -= 15
					}
					if value > best {
						best = value
						chosen = &Move{From: from, To: to}
					}
				}
			}
		}
	}
	if chosen == nil {
		return nil
	}
	action, _ := json.Marshal(map[string]any{"type": "move", "fromX": chosen.From.X, "fromY": chosen.From.Y, "toX": chosen.To.X, "toY": chosen.To.Y})
	return action
}
