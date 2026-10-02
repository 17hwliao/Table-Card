package server

import (
	"testing"
	"time"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
)

// BenchmarkIdleGomokuRoomTick measures work while players are viewing an
// unchanged game. This should not allocate a serialized board every tick.
func BenchmarkIdleGomokuRoomTick(b *testing.B) {
	s := &Server{
		rooms: table.NewRoomManager(), registry: games.NewDefaultRegistry(),
		engines: make(map[string]games.Engine), runtime: make(map[string]*roomRuntime),
		hub: newRoomHub(),
	}
	first := table.Player{ID: "first", Name: "First", Ready: true}
	second := table.Player{ID: "second", Name: "Second", Ready: true}
	room, err := s.rooms.Create(table.GomokuMode, 2, first)
	if err != nil {
		b.Fatal(err)
	}
	if err := room.Join(second); err != nil {
		b.Fatal(err)
	}
	if err := room.SetReady(second.ID, true); err != nil {
		b.Fatal(err)
	}
	if err := room.Start(); err != nil {
		b.Fatal(err)
	}
	code := room.Snapshot().Code
	engine, err := s.configuredEngine(room.Snapshot())
	if err != nil {
		b.Fatal(err)
	}
	s.engines[code] = engine
	s.runtime[code] = newRoomRuntime()
	now := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		s.tick(now)
	}
}
