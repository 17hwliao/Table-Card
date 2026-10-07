package tetris

import (
	"github.com/17hwliao/table-card-independent/internal/table"
	"testing"
	"time"
)

func fixture(t *testing.T, n int) *Engine {
	t.Helper()
	players := make([]table.Player, n)
	for i := range players {
		players[i] = table.Player{ID: string(rune('a' + i))}
	}
	e, err := NewEngine(players)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestIdenticalBagsAndValidSeats(t *testing.T) {
	e := fixture(t, 4)
	for i := 1; i < 4; i++ {
		if e.players[i].Active != e.players[0].Active || e.players[i].Next != e.players[0].Next {
			t.Fatal("opening sequence differs")
		}
	}
	for range 100 {
		a := e.players[0].nextKind()
		for i := 1; i < 4; i++ {
			if e.players[i].nextKind() != a {
				t.Fatal("bag sequence differs")
			}
		}
	}
	if _, err := NewEngine([]table.Player{{ID: "a"}, {ID: "b"}, {ID: "c"}}); err == nil {
		t.Fatal("three seats accepted")
	}
	if _, err := NewEngine([]table.Player{{ID: "a"}, {ID: "a"}}); err == nil {
		t.Fatal("duplicate player accepted")
	}
}
func TestClearRowsAndGhost(t *testing.T) {
	var board Board
	for x := 0; x < Width; x++ {
		board[18][x] = 1
		board[19][x] = 2
	}
	board[17][4] = 3
	if clearRows(&board) != 2 || board[19][4] != 3 || board[0] != [Width]uint8{} {
		t.Fatal("row compression failed")
	}
	p := Ghost(Board{}, Piece{Kind: 2, X: 3, Y: -1})
	if !Fits(Board{}, p) || Fits(Board{}, Piece{Kind: p.Kind, X: p.X, Y: p.Y + 1}) {
		t.Fatal("ghost is not last legal landing")
	}
}
func TestActionsAndSimultaneousTopOut(t *testing.T) {
	e := fixture(t, 2)
	if _, err := e.Apply("a", []byte(`{"type":"hard_drop"}`)); err == nil {
		t.Fatal("countdown bypassed")
	}
	e.startAt = time.Now().Add(-time.Second)
	if _, err := e.Apply("a", []byte(`{"type":"hard_drop"}`)); err != nil {
		t.Fatal(err)
	}
	if e.players[0].Locked != 1 {
		t.Fatal("hard drop not locked")
	}
	if _, err := e.Apply("intruder", []byte(`{"type":"left"}`)); err == nil {
		t.Fatal("unknown player accepted")
	}
	e = fixture(t, 4)
	now := e.startAt.Add(2 * time.Second)
	for i := range e.players {
		p := &e.players[i]
		p.Board[0][0] = 1
		p.Active = Piece{Kind: 2, X: 3, Y: 18}
		p.lastFall = now.Add(-time.Second)
	}
	e.Tick(now, nil) // First contact starts the fixed landing delay.
	if !e.Tick(now.Add(e.lockDelay()), nil) || !e.finished || !e.draw || e.winner != "" {
		t.Fatal("simultaneous elimination not a draw")
	}
}
func TestBotRoundsAndViewIsolation(t *testing.T) {
	e := fixture(t, 4)
	now := e.startAt
	ids := []string{"a", "b", "c", "d"}
	for i := 0; i < 600 && !e.Finished(); i++ {
		now = now.Add(1500 * time.Millisecond)
		e.Tick(now, ids)
	}
	for _, p := range e.players {
		if p.Locked == 0 {
			t.Fatal("bot made no progress")
		}
		if p.Alive && !Fits(p.Board, p.Active) {
			t.Fatal("bot created illegal piece")
		}
	}
	v := e.View("").(Snapshot)
	v.Players[0].Board[0][0] = 7
	if e.players[0].Board[0][0] == 7 {
		t.Fatal("snapshot aliases engine board")
	}
}
