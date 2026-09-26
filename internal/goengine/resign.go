package goengine

import "errors"

func (e *Engine) resign(playerID string) (any, error) {
	g := e.game
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.finished {
		return nil, errors.New("对局已经结束")
	}
	for i, p := range g.players {
		if p.ID == playerID {
			g.winner = g.players[1-i].ID
			g.finished = true
			return g.snapshotLocked(), nil
		}
	}
	return nil, errors.New("玩家不在本局中")
}
