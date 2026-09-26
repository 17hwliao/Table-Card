package internationalchess

import "encoding/json"

// BotAction evaluates legal moves including castling, en passant and promotion.
func (e *Engine) BotAction(playerID string) json.RawMessage {
	g := e.game
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.winner != "" || g.draw || g.players[int(g.turn)-1].ID != playerID {
		return nil
	}
	values := [7]int{0, 100, 320, 330, 500, 900, 20000}
	best := -1 << 30
	var chosen *Move
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			piece := g.position.board[y][x]
			if piece.Side != g.turn {
				continue
			}
			from := Point{x, y}
			for ty := 0; ty < BoardSize; ty++ {
				for tx := 0; tx < BoardSize; tx++ {
					to := Point{tx, ty}
					if !legalMove(g.position, g.turn, from, to, Queen) {
						continue
					}
					next := applyMove(g.position, from, to, Queen)
					value := values[g.position.board[ty][tx].Kind]*10 + (7-abs(tx*2-7)-abs(ty*2-7))*3
					if piece.Kind == Pawn {
						if from.X != to.X && g.position.board[ty][tx].Kind == Empty {
							value += 1000
						}
						if ty == 0 || ty == 7 {
							value += 8000
						}
						if g.turn == White {
							value += (6 - ty) * 8
						} else {
							value += (ty - 1) * 8
						}
					}
					if piece.Kind == King && abs(tx-x) == 2 {
						value += 80
					}
					if inCheck(next.board, opposite(g.turn)) {
						value += 35
						if !hasLegalMove(next, opposite(g.turn)) {
							value += 1000000
						}
					}
					if isAttacked(next.board, to, opposite(g.turn)) {
						value -= values[next.board[ty][tx].Kind] * 9
					}
					if g.positions[positionKey(next, opposite(g.turn))] > 0 {
						value -= 100
					}
					if value > best {
						best = value
						chosen = &Move{From: from, To: to, Promotion: Queen}
					}
				}
			}
		}
	}
	if chosen == nil {
		return nil
	}
	action, _ := json.Marshal(map[string]any{"type": "move", "fromX": chosen.From.X, "fromY": chosen.From.Y, "toX": chosen.To.X, "toY": chosen.To.Y, "promotion": chosen.Promotion})
	return action
}
