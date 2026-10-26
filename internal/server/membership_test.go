package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/table"
)

func TestRoomMembershipAllEntryPathsAndLeave(t *testing.T) {
	s := reviewServer(t)
	h := s.Handler()
	id, token := testIdentity(strings.Repeat("membership", 8))
	other, otherToken := testIdentity(strings.Repeat("host", 16))
	post := func(path, player, secret string, body any) *httptest.ResponseRecorder {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", path, strings.NewReader(string(data)))
		r.Header.Set("X-Player-ID", player)
		r.Header.Set("X-Player-Token", secret)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	request := func(player string) map[string]any {
		return map[string]any{"mode": "garden_relay", "seats": 2, "playerId": player, "name": "Player"}
	}
	a := post("/api/rooms", id, token, request(id))
	b := post("/api/rooms", other, otherToken, request(other))
	if a.Code != 201 || b.Code != 201 {
		t.Fatal("cannot create initial rooms")
	}
	var first, second struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(a.Body.Bytes(), &first) != nil || json.Unmarshal(b.Body.Bytes(), &second) != nil {
		t.Fatal("invalid room response")
	}
	for _, endpoint := range []string{"/api/rooms", "/api/match", "/api/rooms/" + second.Code + "/join"} {
		body := request(id)
		if strings.HasSuffix(endpoint, "/join") {
			body = map[string]any{"id": id, "name": "Player"}
		}
		if w := post(endpoint, id, token, body); w.Code != http.StatusConflict {
			t.Fatalf("duplicate membership %s: %d %s", endpoint, w.Code, w.Body.String())
		}
	}
	if s.rooms.Len() != 2 {
		t.Fatal("rejected requests created extra rooms")
	}
	if w := post("/api/rooms/"+first.Code+"/leave", id, token, map[string]string{"playerId": id}); w.Code >= 400 {
		t.Fatalf("leave failed: %s", w.Body.String())
	}
	if w := post("/api/rooms/"+second.Code+"/join", id, token, map[string]string{"id": id, "name": "Player"}); w.Code >= 400 {
		t.Fatalf("join after leave failed: %s", w.Body.String())
	}
	room := s.socialRoomLocked(id)
	if room == nil || room.Snapshot().Code != second.Code {
		t.Fatal("room chat no longer follows current membership")
	}
	for _, u := range s.presenceLocked() {
		if u.PlayerID == id && u.RoomCode != second.Code {
			t.Fatal("presence points at a stale room")
		}
	}
}

func TestConcurrentRoomCreationHasOneMembership(t *testing.T) {
	s := reviewServer(t)
	id, token := testIdentity(strings.Repeat("concurrent-room", 6))
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	for range cap(codes) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/rooms", strings.NewReader(`{"mode":"garden_relay","seats":2,"playerId":"`+id+`","name":"Player"}`))
			r.Header.Set("X-Player-ID", id)
			r.Header.Set("X-Player-Token", token)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	successes := 0
	for code := range codes {
		if code == 201 {
			successes++
		} else if code != 409 {
			t.Fatalf("unexpected status %d", code)
		}
	}
	if successes != 1 || s.rooms.Len() != 1 {
		t.Fatalf("concurrent requests made %d successful creations / %d rooms", successes, s.rooms.Len())
	}
}

func TestMembershipAfterInProgressBotTakeover(t *testing.T) {
	s := reviewServer(t)
	h := s.Handler()
	id, token := testIdentity(strings.Repeat("departing", 8))
	other, otherToken := testIdentity(strings.Repeat("remaining", 8))
	post := func(path, player, secret string, body any) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", path, strings.NewReader(string(data)))
		r.Header.Set("X-Player-ID", player)
		r.Header.Set("X-Player-Token", secret)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	created := post("/api/rooms", id, token, map[string]any{"mode": "garden_relay", "seats": 2, "playerId": id, "name": "Leaving"})
	var room table.Snapshot
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &room) != nil {
		t.Fatalf("create failed: %d %s", created.Code, created.Body.String())
	}
	base := "/api/rooms/" + room.Code
	for _, step := range []struct {
		path, player, secret string
		body                 any
	}{
		{base + "/join", other, otherToken, map[string]string{"id": other, "name": "Remaining"}},
		{base + "/ready", id, token, map[string]any{"playerId": id, "ready": true}},
		{base + "/ready", other, otherToken, map[string]any{"playerId": other, "ready": true}},
		{base + "/start", id, token, map[string]string{}},
		{base + "/leave", id, token, map[string]string{"playerId": id}},
	} {
		if w := post(step.path, step.player, step.secret, step.body); w.Code >= 400 {
			t.Fatalf("%s failed: %d %s", step.path, w.Code, w.Body.String())
		}
	}
	old, ok := s.rooms.Find(room.Code)
	if !ok || !old.Snapshot().Players[0].Bot {
		t.Fatal("departing player did not retain a bot-controlled seat")
	}
	if w := post(base+"/action", id, token, map[string]any{"playerId": id, "action": map[string]string{"type": "resign"}}); w.Code != http.StatusForbidden {
		t.Fatalf("departed player still controls old seat: %d %s", w.Code, w.Body.String())
	}
	matched := post("/api/match", id, token, map[string]any{"mode": "garden_relay", "seats": 2, "playerId": id, "name": "Leaving"})
	var current table.Snapshot
	if matched.Code != 201 || json.Unmarshal(matched.Body.Bytes(), &current) != nil || current.Code == room.Code {
		t.Fatalf("bot seat incorrectly blocks re-entry: %d %s", matched.Code, matched.Body.String())
	}
	for _, user := range s.presenceLocked() {
		if user.PlayerID == id && user.RoomCode != current.Code {
			t.Fatal("presence still selects the previous bot-controlled room")
		}
	}
	if selected := s.socialRoomLocked(id); selected == nil || selected.Snapshot().Code != current.Code {
		t.Fatal("room chat still selects the previous bot-controlled room")
	}
}

func TestConcurrentCreateJoinMatchHaveOneMembership(t *testing.T) {
	s := reviewServer(t)
	h := s.Handler()
	id, token := testIdentity(strings.Repeat("mixed-entry", 8))
	room, err := s.rooms.Create(table.GardenMode, 2, table.Player{ID: "waiting-host", Name: "Host"})
	if err != nil {
		t.Fatal(err)
	}
	s.runtime[room.Snapshot().Code] = newRoomRuntime()
	start := make(chan struct{})
	codes := make(chan int, 12)
	var wg sync.WaitGroup
	for i := range cap(codes) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			endpoint := []string{"/api/rooms", "/api/match", "/api/rooms/" + room.Snapshot().Code + "/join"}[i%3]
			payload := map[string]any{"mode": "garden_relay", "seats": 2, "playerId": id, "name": "Player"}
			if i%3 == 2 {
				payload = map[string]any{"id": id, "name": "Player"}
			}
			body, _ := json.Marshal(payload)
			r := httptest.NewRequest("POST", endpoint, strings.NewReader(string(body)))
			r.Header.Set("X-Player-ID", id)
			r.Header.Set("X-Player-Token", token)
			w := httptest.NewRecorder()
			<-start
			h.ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)
	successes := 0
	for code := range codes {
		if code == 200 || code == 201 {
			successes++
		} else if code != http.StatusConflict {
			t.Fatalf("unexpected mixed-entry status %d", code)
		}
	}
	memberships := 0
	for _, candidate := range s.rooms.List() {
		if isHumanRoomPlayer(candidate.Snapshot(), id) {
			memberships++
		}
	}
	if successes != 1 || memberships != 1 {
		t.Fatalf("mixed requests made %d successes / %d memberships", successes, memberships)
	}
}
