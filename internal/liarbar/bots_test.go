package liarbar_test

import (
	"encoding/json"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
)

// Exercise the public engine contract used by the timed room scheduler.
func TestCardBotsFinishWithOnlyLegalScheduledMoves(t *testing.T) {
	for _, mode := range []table.Mode{table.LandlordMode, table.LiarBarMode} {
		t.Run(string(mode), func(t *testing.T) {
			players := []table.Player{{ID: "a", Name: "A", Bot: true}, {ID: "b", Name: "B", Bot: true}, {ID: "c", Name: "C", Bot: true}}
			if mode == table.LiarBarMode {
				players = append(players, table.Player{ID: "d", Name: "D", Bot: true})
			}
			e, err := games.NewDefaultRegistry().New(mode, players)
			if err != nil {
				t.Fatal(err)
			}
			bot := e.(interface{ BotAction(string) json.RawMessage })
			for step := 0; step < 1000; step++ {
				var actor string
				var action json.RawMessage
				for _, p := range players {
					if candidate := bot.BotAction(p.ID); len(candidate) > 0 {
						if actor != "" {
							t.Fatal("more than one current actor")
						}
						actor = p.ID
						action = candidate
					}
				}
				if actor == "" {
					data, _ := json.Marshal(e.View(players[0].ID))
					var end struct {
						Phase int `json:"phase"`
					}
					_ = json.Unmarshal(data, &end)
					if (mode == table.LiarBarMode && end.Phase != 1) || (mode == table.LandlordMode && end.Phase != 3) {
						t.Fatalf("bots stalled: %s", data)
					}
					return
				}
				if _, err := e.Apply(actor, action); err != nil {
					t.Fatalf("step %d illegal bot %s: %v", step, action, err)
				}
			}
			t.Fatal("bots failed to end game within scheduling limit")
		})
	}
}
