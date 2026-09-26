package gomoku

import (
	"encoding/json"
	"github.com/17hwliao/table-card-independent/internal/table"
	"testing"
)

func TestBotWinsBeforeBlocking(t *testing.T) {
	e, _ := NewEngine([]table.Player{{ID: "a"}, {ID: "b"}})
	for x := 0; x < 4; x++ {
		e.game.board[7][x] = Black
		e.game.board[8][x] = White
	}
	var move struct{ X, Y int }
	json.Unmarshal(e.BotAction("a"), &move)
	if move.Y != 7 || move.X != 4 {
		t.Fatalf("must take winning move, got %+v", move)
	}
	if _, err := e.Apply("a", e.BotAction("a")); err != nil {
		t.Fatal(err)
	}
	if e.game.Snapshot().Winner != "a" {
		t.Fatal("winning move not applied")
	}
	if e.BotAction("b") != nil {
		t.Fatal("finished bot acted")
	}
}
func TestBotBlocksFour(t *testing.T) {
	e, _ := NewEngine([]table.Player{{ID: "a"}, {ID: "b"}})
	for x := 0; x < 4; x++ {
		e.game.board[7][x] = White
	}
	var move struct{ X, Y int }
	json.Unmarshal(e.BotAction("a"), &move)
	if move.Y != 7 || move.X != 4 {
		t.Fatalf("must block, got %+v", move)
	}
}
