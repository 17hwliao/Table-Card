package netclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/17hwliao/table-card-independent/internal/garden"
	"github.com/coder/websocket"
)

func TestRoomReadAcceptsCapacitySnapshotAndRejectsOversizedMessage(t *testing.T) {
	// This is a supported-capacity boundary, rather than a claim that an
	// ordinary bot match necessarily fills every projectile and zombie slot.
	game := garden.Snapshot{Players: make([]garden.Lawn, 2)}
	for seat := range game.Players {
		lawn := &game.Players[seat]
		lawn.ID = string(rune('a' + seat))
		lawn.Name, lawn.Alive = lawn.ID, true
		for row := range garden.Rows {
			for col := range garden.Cols {
				lawn.Grid[row][col] = garden.Plant{Kind: garden.Repeater, HP: garden.Plants[garden.Repeater].HP}
			}
		}
		for i := range garden.MaxZombies / 2 {
			def := garden.Zombies[garden.Bucket]
			lawn.Zombies = append(lawn.Zombies, garden.Zombie{ID: uint64(seat*60 + i + 1), Kind: garden.Bucket, Row: i % garden.Rows, X: garden.Cols * garden.Cell, HP: def.HP, Armor: def.Armor, SummonWait: 90})
		}
		for i := range garden.MaxShots {
			lawn.Shots = append(lawn.Shots, garden.Shot{Row: i % garden.Rows, X: 500 + (i%80)*100})
		}
	}
	capacityPayload, err := json.Marshal(map[string]any{"type": "state", "game": game})
	if err != nil {
		t.Fatal(err)
	}
	if len(capacityPayload) <= 32768 || len(capacityPayload) > roomMessageReadLimit {
		t.Fatalf("capacity fixture size %d no longer exercises the previous read boundary", len(capacityPayload))
	}
	for _, tc := range []struct {
		name    string
		payload []byte
		tooBig  bool
	}{
		{"supported capacity", capacityPayload, false},
		{"over configured limit", []byte(`{"type":"state","name":"` + strings.Repeat("x", roomMessageReadLimit) + `"}`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				defer cancel()
				_ = conn.Write(ctx, websocket.MessageText, tc.payload)
				_, _, _ = conn.Read(ctx)
			}))
			defer server.Close()
			client := New(server.URL)
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := client.Connect(ctx, "room", "a"); err != nil {
				t.Fatal(err)
			}
			got, err := client.Read()
			if tc.tooBig {
				if !errors.Is(err, websocket.ErrMessageTooBig) {
					t.Fatalf("oversized message error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var received garden.Snapshot
			if json.Unmarshal(got.Game, &received) != nil || len(received.Players) != 2 || len(received.Players[1].Shots) != garden.MaxShots || len(received.Players[0].Zombies) != garden.MaxZombies/2 {
				t.Fatal("capacity snapshot was truncated or did not decode")
			}
		})
	}
}
