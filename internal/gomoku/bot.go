package gomoku

import "encoding/json"

// BotAction chooses a win, blocks an immediate loss, then builds open lines.
func (e *Engine) BotAction(playerID string) json.RawMessage {
	g := e.game
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.winner != "" || g.draw || g.players[int(g.turn)-1].ID != playerID {
		return nil
	}
	best, xbest, ybest := -1<<30, 7, 7
	other := uint8(3) - g.turn
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if g.board[y][x] != Empty {
				continue
			}
			board := g.board
			board[y][x] = g.turn
			value := lineValue(board, x, y, g.turn)*2 + lineValue(board, x, y, other)*2 - (abs(x-7) + abs(y-7))
			if hasFive(board, x, y, g.turn) {
				value += 10000000
			}
			board[y][x] = other
			if hasFive(board, x, y, other) {
				value += 1000000
			}
			if value > best {
				best, xbest, ybest = value, x, y
			}
		}
	}
	action, _ := json.Marshal(map[string]any{"type": "place", "x": xbest, "y": ybest})
	return action
}
func lineValue(board [BoardSize][BoardSize]uint8, x, y int, side uint8) int {
	total := 0
	for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		count, open := 1, 0
		for _, sign := range []int{-1, 1} {
			for step := 1; step < 5; step++ {
				nx, ny := x+d[0]*step*sign, y+d[1]*step*sign
				if nx < 0 || ny < 0 || nx >= BoardSize || ny >= BoardSize {
					break
				}
				if board[ny][nx] == Empty {
					open++
					break
				}
				if board[ny][nx] != side {
					break
				}
				count++
			}
		}
		if open > 0 {
			weights := []int{0, 1, 10, 100, 10000, 100000}
			if count > 5 {
				count = 5
			}
			total += weights[count] * open
		}
	}
	return total
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
