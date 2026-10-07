package chinesechess

type positionOccurrence struct{ first, count int }
type checkEvent struct {
	side  Side
	check bool
}

func positionKey(board [Ranks][Files]Piece, side Side) string {
	var key [Ranks*Files + 1]byte
	for y := range board {
		for x, piece := range board[y] {
			key[y*Files+x] = byte(piece.Side)*8 + byte(piece.Kind)
		}
	}
	key[Ranks*Files] = byte(side)
	return string(key[:])
}

// This is an explicit casual-table rule, not a full chase/capture arbiter.
// A lone perpetual checking side loses; other third repetitions are draws.
func (g *Game) finishRepetition(first int) {
	allCheck := [3]bool{false, true, true}
	hadMove := [3]bool{}
	for _, event := range g.history[first:] {
		hadMove[event.side] = true
		allCheck[event.side] = allCheck[event.side] && event.check
	}
	red := hadMove[Red] && allCheck[Red]
	black := hadMove[Black] && allCheck[Black]
	if red != black {
		loser := Red
		if black {
			loser = Black
		}
		g.winner = g.players[2-int(loser)].ID
		g.drawReason = "三次重复：单方长将判负（休闲房规）"
	} else {
		g.draw = true
		g.drawReason = "三次重复局面（休闲房规）"
	}
}
