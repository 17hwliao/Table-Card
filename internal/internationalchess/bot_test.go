package internationalchess

import (
	"github.com/17hwliao/table-card-independent/internal/table"
	"testing"
)

func TestBotPlaysLegalGame(t *testing.T) {
	e, err := NewEngine([]table.Player{{ID: "a"}, {ID: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if e.BotAction("b") != nil {
		t.Fatal("bot acted out of turn")
	}
	for i := 0; i < 60; i++ {
		s := e.game.Snapshot()
		if s.Winner != "" || s.Draw {
			break
		}
		id := []string{"a", "b"}[int(s.Turn)-1]
		action := e.BotAction(id)
		if action == nil {
			t.Fatalf("no action at ply %d", i)
		}
		if _, err := e.Apply(id, action); err != nil {
			t.Fatalf("ply %d: %s: %v", i, action, err)
		}
	}
}
