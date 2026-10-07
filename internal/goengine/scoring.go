package goengine

import "errors"

// Dead groups are a shared proposal: any edit invalidates both confirmations.
// Neither player can remove stones and finalize a result alone.
func (g *Game) scoringAction(id, action string, x, y int) (Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	seat := -1
	for i, p := range g.players {
		if p.ID == id {
			seat = i
		}
	}
	if seat < 0 || !g.scoring || g.finished {
		return Snapshot{}, errors.New("当前不能操作数子确认")
	}
	switch action {
	case "mark_dead":
		if !inBoard(x, y) || g.board[y][x] == Empty {
			return Snapshot{}, errors.New("请选择一组棋子标记或取消死子")
		}
		var scratch [BoardSize * BoardSize][2]int
		group, _ := collectInto(g.board, x, y, &scratch)
		mark := !g.dead[y][x]
		for _, p := range group {
			g.dead[p[1]][p[0]] = mark
		}
		g.accepted = [2]bool{}
	case "accept_score":
		g.accepted[seat] = true
		if g.accepted[0] && g.accepted[1] {
			g.finished = true
			g.scoring = false
		}
	case "resume":
		g.scoring = false
		g.passes = 0
		g.dead = [BoardSize][BoardSize]bool{}
		g.accepted = [2]bool{}
	default:
		return Snapshot{}, errors.New("无效的数子操作")
	}
	return g.snapshotLocked(), nil
}
