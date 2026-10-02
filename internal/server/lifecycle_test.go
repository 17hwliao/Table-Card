package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
)

func TestResultRecordedOnWinningActionWithoutIdlePolling(t *testing.T) {
	t.Setenv("TABLE_CARD_DATA_DIR", t.TempDir())
	s := &Server{
		rooms: table.NewRoomManager(), registry: games.NewDefaultRegistry(),
		engines: make(map[string]games.Engine), runtime: make(map[string]*roomRuntime),
		hub: newRoomHub(), stats: newStatStore(),
	}
	first := table.Player{ID: "first", Name: "First", Ready: true}
	second := table.Player{ID: "second", Name: "Second"}
	room, err := s.rooms.Create(table.GomokuMode, 2, first)
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Join(second); err != nil {
		t.Fatal(err)
	}
	if err := room.SetReady(second.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := room.Start(); err != nil {
		t.Fatal(err)
	}
	snapshot := room.Snapshot()
	engine, err := s.configuredEngine(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	s.engines[snapshot.Code] = engine
	s.runtime[snapshot.Code] = newRoomRuntime()
	for x := 0; x < 5; x++ {
		for _, move := range []struct {
			id string
			y  int
		}{{first.ID, 0}, {second.ID, 1}} {
			if x == 4 && move.id == second.ID {
				break
			}
			action := json.RawMessage(fmt.Sprintf(`{"type":"place","x":%d,"y":%d}`, x, move.y))
			if _, err := s.applyLocked(snapshot.Code, room, engine, move.id, action); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !s.runtime[snapshot.Code].finished {
		t.Fatal("winning move did not mark the room finished")
	}
	before := s.stats.entries[string(table.GomokuMode)+"/"+first.ID]
	if before.Wins != 1 {
		t.Fatalf("winner recorded %d wins, want 1", before.Wins)
	}
	for range 3 {
		s.tick(time.Now())
	}
	after := s.stats.entries[string(table.GomokuMode)+"/"+first.ID]
	if after != before {
		t.Fatalf("idle tick changed completed match statistic: before=%+v after=%+v", before, after)
	}
}

type roundTransitionEngine struct {
	finished bool
	round    int
}

func (e *roundTransitionEngine) Mode() table.Mode { return table.UNOMode }
func (e *roundTransitionEngine) View(string) any {
	winner := ""
	if e.finished {
		winner = "first"
	}
	return map[string]any{
		"finished": e.finished, "winner": winner, "round": e.round,
		"players": []map[string]any{{"id": "first", "score": e.round * 20}, {"id": "second", "score": 0}},
	}
}
func (e *roundTransitionEngine) Apply(_ string, payload json.RawMessage) (any, error) {
	var action struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &action); err != nil {
		return nil, err
	}
	switch action.Type {
	case "finish":
		e.finished = true
	case "next_round":
		e.finished = false
		e.round++
	}
	return e.View(""), nil
}

func TestCompletedRoundCanResumeBotScheduling(t *testing.T) {
	t.Setenv("TABLE_CARD_DATA_DIR", t.TempDir())
	s := &Server{runtime: make(map[string]*roomRuntime), stats: newStatStore()}
	// Use a real room to preserve the same snapshot path as the HTTP server.
	manager := table.NewRoomManager()
	room, err := manager.Create(table.UNOMode, 2, table.Player{ID: "first", Name: "First"})
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Join(table.Player{ID: "second", Name: "Second"}); err != nil {
		t.Fatal(err)
	}
	code := room.Snapshot().Code
	s.runtime[code] = newRoomRuntime()
	engine := &roundTransitionEngine{round: 1}
	if _, err := s.applyLocked(code, room, engine, "first", json.RawMessage(`{"type":"finish"}`)); err != nil {
		t.Fatal(err)
	}
	if !s.runtime[code].finished {
		t.Fatal("finished round was not cached")
	}
	if _, err := s.applyLocked(code, room, engine, "first", json.RawMessage(`{"type":"next_round"}`)); err != nil {
		t.Fatal(err)
	}
	if s.runtime[code].finished {
		t.Fatal("next round remained marked finished, so bots would never play")
	}
}

func TestEveryModeReportsCompletionWithoutSerializingGameState(t *testing.T) {
	registry := games.NewDefaultRegistry()
	for _, mode := range modes {
		players := make([]table.Player, mode.MaxSeats)
		for i := range players {
			players[i] = table.Player{ID: fmt.Sprintf("player-%d", i), Name: fmt.Sprintf("Player %d", i)}
		}
		engine, err := registry.New(mode.ID, players)
		if err != nil {
			t.Fatalf("%s: %v", mode.ID, err)
		}
		checker, ok := engine.(interface{ Finished() bool })
		if !ok {
			t.Fatalf("%s cannot report game completion without a full JSON snapshot", mode.ID)
		}
		if checker.Finished() {
			t.Fatalf("%s unexpectedly starts finished", mode.ID)
		}
	}
}
