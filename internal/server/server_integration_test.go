package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/coder/websocket"
)

func testIdentity(token string) (string, string) {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:16]), token
}

func TestAuthenticatedBotRoomLifecycle(t *testing.T) {
	t.Setenv("TABLE_CARD_DATA_DIR", t.TempDir())
	s := New()
	httpServer := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		httpServer.Close()
		s.Close()
	})

	playerID, token := testIdentity(strings.Repeat("a", 64))
	post := func(path, body string, authorized bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, httpServer.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if authorized {
			req.Header.Set("X-Player-ID", playerID)
			req.Header.Set("X-Player-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	create := post("/api/rooms", `{"mode":"liar_bar","seats":4,"bots":3,"playerId":"`+playerID+`","name":"Test player"}`, true)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create room status = %d, want %d", create.StatusCode, http.StatusCreated)
	}
	var room table.Snapshot
	if err := json.NewDecoder(create.Body).Decode(&room); err != nil {
		t.Fatal(err)
	}
	_ = create.Body.Close()
	if room.Code == "" || len(room.Players) != 4 || room.Phase != table.Waiting {
		t.Fatalf("unexpected new bot room snapshot: %+v", room)
	}

	ready := post("/api/rooms/"+room.Code+"/ready", `{"playerId":"`+playerID+`","ready":true}`, true)
	if ready.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", ready.StatusCode, http.StatusOK)
	}
	_ = ready.Body.Close()

	start := post("/api/rooms/"+room.Code+"/start", `{}`, true)
	if start.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d, want %d", start.StatusCode, http.StatusOK)
	}
	_ = start.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + room.Code + "/ws?playerId=" + playerID
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"X-Player-Token": []string{token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")
	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Type string `json:"type"`
		Game struct {
			Hand []any `json:"hand"`
		} `json:"game"`
	}
	if err := json.Unmarshal(payload, &state); err != nil {
		t.Fatal(err)
	}
	if state.Type != "state" || len(state.Game.Hand) == 0 {
		t.Fatalf("unexpected private game state envelope: %s", payload)
	}

	unauthorized := post("/api/rooms/"+room.Code+"/action", `{"playerId":"`+playerID+`","action":{"type":"challenge"}}`, false)
	if unauthorized.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized action status = %d, want %d", unauthorized.StatusCode, http.StatusForbidden)
	}
	_ = unauthorized.Body.Close()

	statsRequest, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/stats?mode=liar_bar&playerId="+playerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	statsResponse, err := http.DefaultClient.Do(statsRequest)
	if err != nil {
		t.Fatal(err)
	}
	if statsResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized stats status = %d, want %d", statsResponse.StatusCode, http.StatusForbidden)
	}
	_ = statsResponse.Body.Close()

	leave := post("/api/rooms/"+room.Code+"/leave", `{"playerId":"`+playerID+`"}`, true)
	if leave.StatusCode != http.StatusOK {
		t.Fatalf("leave status = %d, want %d", leave.StatusCode, http.StatusOK)
	}
	_ = leave.Body.Close()
	readCtx, readCancel := context.WithTimeout(context.Background(), time.Second)
	defer readCancel()
	if _, _, err := conn.Read(readCtx); err == nil {
		t.Fatal("WebSocket remained open after the player left the room")
	}
	if _, exists := s.rooms.Find(room.Code); exists {
		t.Fatal("room remains after its final human player leaves")
	}
}
