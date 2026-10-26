package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/table"
)

func TestLegacyRoomChatDeliveredOnceAcrossRoomAndSocial(t *testing.T) {
	for _, slowSocial := range []bool{false, true} {
		name := "social recipient"
		if slowSocial {
			name = "slow social falls back to room"
		}
		t.Run(name, func(t *testing.T) {
			s := reviewServer(t)
			room, err := s.rooms.Create(table.GardenMode, 2, table.Player{ID: "legacy", Name: "Legacy"})
			if err != nil {
				t.Fatal(err)
			}
			if err := room.Join(table.Player{ID: "modern", Name: "Modern"}); err != nil {
				t.Fatal(err)
			}
			peer := func(id string) *socketPeer {
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				return &socketPeer{roomCode: room.Snapshot().Code, playerID: id, name: id, ctx: ctx, cancel: cancel, out: make(chan []byte, 8)}
			}
			legacy, modernRoom, modernSocial := peer("legacy"), peer("modern"), peer("modern")
			s.hub.add(legacy)
			s.hub.add(modernRoom)
			if slowSocial {
				modernSocial.out = make(chan []byte)
			}
			s.social.peers[modernSocial] = struct{}{}
			s.handleSocketMessage(legacy, room, socketRequest{Type: "chat", Text: "hello"})
			if len(legacy.out) != 1 {
				t.Fatal("room-only sender did not receive its chat")
			}
			if got := len(modernRoom.out) + len(modernSocial.out); got != 1 {
				t.Fatalf("modern recipient got %d copies across its two streams, want one", got)
			}
			var raw []byte
			if slowSocial {
				raw = <-modernRoom.out
			} else {
				if len(modernRoom.out) != 0 {
					t.Fatal("live social recipient also received room-stream chat")
				}
				raw = <-modernSocial.out
			}
			var received struct{ Type, Text, RoomCode string }
			if json.Unmarshal(raw, &received) != nil || received.Type != "chat" || received.Text != "hello" || received.RoomCode != room.Snapshot().Code {
				t.Fatal("chat metadata was lost")
			}
		})
	}
}
