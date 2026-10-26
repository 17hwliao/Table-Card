package games

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/table"
)

func reviewPlayers(mode table.Mode) []table.Player {
	n := 2
	if mode == table.LandlordMode {
		n = 3
	}
	if mode == table.LiarBarMode || mode == table.MahjongMode || mode == table.UNOMode {
		n = 4
	}
	p := make([]table.Player, n)
	for i := range p {
		p[i] = table.Player{ID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("P%d", i)}
	}
	return p
}

var reviewModes = []table.Mode{table.LandlordMode, table.LiarBarMode, table.MahjongMode, table.ChessMode, table.WesternChessMode, table.GomokuMode, table.GoMode, table.UNOMode, table.TetrisMode, table.GardenMode}

func TestReviewAllRoomModesRejectForeignActions(t *testing.T) {
	registry := NewDefaultRegistry()
	for _, mode := range reviewModes {
		t.Run(string(mode), func(t *testing.T) {
			e, err := registry.New(mode, reviewPlayers(mode))
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(e.View("p0"))
			for _, raw := range []string{`null`, `[]`, `{"type":"resign"}`, `{"type":"next_round"}`, `{"type":"plant","row":0,"col":0,"plant":1}`, `{"type":"play","cardIds":[0]}`} {
				if _, err := e.Apply("intruder", []byte(raw)); err == nil {
					t.Fatalf("foreign action accepted %s", raw)
				}
			}
			after, _ := json.Marshal(e.View("p0"))
			if string(before) != string(after) {
				t.Fatal("rejected foreign action mutated game")
			}
		})
	}
}

func TestReviewTurnBasedBotsMakeLegalProgress(t *testing.T) {
	r := NewDefaultRegistry()
	for _, mode := range reviewModes {
		if mode == table.TetrisMode || mode == table.GardenMode {
			continue
		}
		t.Run(string(mode), func(t *testing.T) {
			players := reviewPlayers(mode)
			e, err := r.New(mode, players)
			if err != nil {
				t.Fatal(err)
			}
			bot, ok := e.(interface{ BotAction(string) json.RawMessage })
			if !ok {
				t.Fatal("bot interface missing")
			}
			for step := 0; step < 120; step++ {
				if e.(interface{ Finished() bool }).Finished() {
					return
				}
				progress := false
				for _, p := range players {
					raw := bot.BotAction(p.ID)
					if len(raw) == 0 {
						continue
					}
					if _, err := e.Apply(p.ID, raw); err != nil {
						t.Fatalf("step%d %s bot produced illegal action %s: %v", step, p.ID, raw, err)
					}
					progress = true
					break
				}
				if !progress {
					t.Fatal("unfinished round has no bot action")
				}
			}
		})
	}
}
