package server

import (
	"fmt"
	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
	"testing"
	"time"
)

func TestRepeatedAllModeRoomsReleaseEnginesAndRuntime(t *testing.T) {
	t.Setenv("TABLE_CARD_DATA_DIR", t.TempDir())
	s := &Server{rooms: table.NewRoomManager(), registry: games.NewDefaultRegistry(), engines: make(map[string]games.Engine), runtime: make(map[string]*roomRuntime), hub: newRoomHub(), stats: newStatStore()}
	for round := 0; round < 30; round++ {
		for _, mode := range []table.Mode{table.LandlordMode, table.LiarBarMode, table.MahjongMode, table.ChessMode, table.WesternChessMode, table.GomokuMode, table.GoMode, table.UNOMode, table.TetrisMode} {
			seats := 2
			if mode == table.LandlordMode {
				seats = 3
			}
			if mode == table.LiarBarMode || mode == table.MahjongMode || mode == table.UNOMode || mode == table.TetrisMode {
				seats = 4
			}
			owner := table.Player{ID: "owner", Name: "Owner", Ready: true}
			room, err := s.rooms.Create(mode, seats, owner)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i < seats; i++ {
				p := table.Player{ID: fmt.Sprintf("bot%d", i), Name: "Bot", Bot: true}
				room.Join(p)
				room.SetReady(p.ID, true)
			}
			if err := room.Start(); err != nil {
				t.Fatal(err)
			}
			snap := room.Snapshot()
			e, err := s.configuredEngine(snap)
			if err != nil {
				t.Fatal(err)
			}
			s.engines[snap.Code] = e
			s.runtime[snap.Code] = newRoomRuntime()
			if err := s.leaveRoom(room, owner.ID); err != nil {
				t.Fatal(err)
			}
			if len(s.rooms.List()) != 0 || len(s.engines) != 0 || len(s.runtime) != 0 || len(s.hub.peers) != 0 {
				t.Fatal("room retained resources after last human left")
			}
		}
	}
}

func TestIdleReleaseIsBoundedAndSkipsLiveRooms(t *testing.T) {
	s := &Server{rooms: table.NewRoomManager()}
	now := time.Now()
	if s.takeIdleRelease(now) {
		t.Fatal("reclaimed without a retired room")
	}
	room, err := s.rooms.Create(table.GoMode, 2, table.Player{ID: "a", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	s.idleRelease.Store(true)
	if s.takeIdleRelease(now) {
		t.Fatal("reclaimed with a live room")
	}
	s.rooms.Remove(room.Snapshot().Code)
	if !s.takeIdleRelease(now) || s.takeIdleRelease(now) {
		t.Fatal("idle release must run exactly once")
	}
	s.idleRelease.Store(true)
	if s.takeIdleRelease(now.Add(time.Second)) || !s.takeIdleRelease(now.Add(10*time.Second)) {
		t.Fatal("idle release cooldown was not respected")
	}
}
