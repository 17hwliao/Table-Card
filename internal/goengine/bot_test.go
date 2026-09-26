package goengine

import (
	"github.com/17hwliao/table-card-independent/internal/table"
	"testing"
)

func TestSuperkoAndKomi(t *testing.T) {
	g, _ := New([]table.Player{{ID: "a"}, {ID: "b"}})
	if g.Snapshot().WhiteScore != 7.5 {
		t.Fatal("wrong komi")
	}
	g.board[1][1] = White
	for _, p := range [][2]int{{0, 1}, {1, 0}, {2, 1}} {
		g.board[p[1]][p[0]] = Black
	}
	for _, p := range [][2]int{{0, 2}, {2, 2}, {1, 3}} {
		g.board[p[1]][p[0]] = White
	}
	g.history = map[[BoardSize][BoardSize]uint8]bool{g.board: true}
	if _, err := g.Play("a", 1, 2, false); err != nil {
		t.Fatal(err)
	}
	before := g.board
	if _, err := g.Play("b", 1, 1, false); err == nil {
		t.Fatal("ko recapture accepted")
	}
	if g.board != before {
		t.Fatal("illegal move changed board")
	}
	// Older recorded positions are also forbidden, not only the last two turns.
	g2, _ := New([]table.Player{{ID: "a"}, {ID: "b"}})
	historical := g2.board
	historical[5][5] = Black
	g2.history[historical] = true
	if _, err := g2.Play("a", 5, 5, false); err == nil {
		t.Fatal("historical position accepted")
	}
}
func TestBotMovesAreLegal(t *testing.T) {
	e, _ := NewEngine([]table.Player{{ID: "a"}, {ID: "b"}})
	for i := 0; i < 40; i++ {
		s := e.game.Snapshot()
		if s.Finished {
			break
		}
		a := e.BotAction(s.TurnPlayer)
		if a == nil {
			t.Fatal("no action")
		}
		if _, err := e.Apply(s.TurnPlayer, a); err != nil {
			t.Fatalf("%s: %v", a, err)
		}
	}
}
