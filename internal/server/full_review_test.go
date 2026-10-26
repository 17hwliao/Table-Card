package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/coder/websocket"
)

func reviewServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("TABLE_CARD_DATA_DIR", t.TempDir())
	return &Server{pokemonSocial: newPokemonSocialStore(), rooms: table.NewRoomManager(), registry: games.NewDefaultRegistry(), engines: map[string]games.Engine{}, runtime: map[string]*roomRuntime{}, hub: newRoomHub(), social: newSocialHub(), stats: newStatStore(), stop: make(chan struct{})}
}
func reviewPeer(t *testing.T, s *Server, name string) *socketPeer {
	t.Helper()
	id, _ := testIdentity(name)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := &socketPeer{playerID: id, name: name, ctx: ctx, cancel: cancel, out: make(chan []byte, 256)}
	s.social.peers[p] = struct{}{}
	s.social.modes[id] = table.PokemonMode
	return p
}
func reviewRequest(s *Server, p *socketPeer, typ string, data any, target string) {
	raw, _ := json.Marshal(data)
	s.handleSocialMessage(p, socialRequest{Type: typ, TargetID: target, Data: raw})
}
func reviewSync(t *testing.T, s *Server, p *socketPeer, species int) {
	t.Helper()
	g := pokemon.New(p.name)
	if err := g.Command("starter 1"); err != nil {
		t.Fatal(err)
	}
	g.Party[0].Species = species
	if err := pokemon.NormalizeData(&g.State); err != nil {
		t.Fatal(err)
	}
	reviewRequest(s, p, "pokemon_sync", map[string]any{"state": g.State}, "")
	if !s.pokemonSocial.ready[p.playerID] {
		t.Fatal("sync did not ready profile")
	}
}
func TestReviewPokemonTradeStakeAndRestart(t *testing.T) {
	s := reviewServer(t)
	a, b := reviewPeer(t, s, "A"), reviewPeer(t, s, "B")
	reviewSync(t, s, a, 64)
	reviewSync(t, s, b, 19)
	x := s.pokemonSocial
	reviewRequest(s, a, "pokemon_invite", map[string]any{"kind": "trade", "slots": [2]int{0, 0}}, b.playerID)
	var offer pokemonOffer
	for _, o := range x.offers {
		offer = o
	}
	if offer.Kind != "trade" {
		t.Fatal("no trade offer")
	}
	reviewRequest(s, b, "pokemon_reply", map[string]any{"id": offer.ID}, "")
	if x.profiles[a.playerID].State.Party[0].Species != 19 || x.profiles[b.playerID].State.Party[0].Species != 65 {
		t.Fatal("trade or trade evolution failed")
	}
	reviewRequest(s, a, "pokemon_invite", map[string]any{"kind": "duel"}, b.playerID)
	for _, o := range x.offers {
		offer = o
	}
	reviewRequest(s, b, "pokemon_reply", map[string]any{"id": offer.ID}, "")
	if x.profiles[a.playerID].State.Money != 3700 || !x.profiles[a.playerID].Escrow || x.matches[a.playerID] == nil {
		t.Fatal("stake not reserved")
	}
	reviewRequest(s, a, "pokemon_cancel", nil, "")
	if x.profiles[a.playerID].State.Money != 3700 || x.profiles[b.playerID].State.Money != 4300 || len(x.matches) != 0 {
		t.Fatal("forfeit payout incorrect")
	}
	restarted := newPokemonSocialStore()
	if restarted.loadErr != nil || restarted.profiles[b.playerID].State.Money != 4300 {
		t.Fatalf("settled persistence failed: %v", restarted.loadErr)
	}
	reviewRequest(s, a, "pokemon_invite", map[string]any{"kind": "duel"}, b.playerID)
	for _, o := range x.offers {
		offer = o
	}
	reviewRequest(s, b, "pokemon_reply", map[string]any{"id": offer.ID}, "")
	restarted = newPokemonSocialStore()
	if restarted.loadErr != nil || restarted.profiles[a.playerID].State.Money != 3700 || restarted.profiles[b.playerID].State.Money != 4300 || restarted.profiles[a.playerID].Escrow {
		t.Fatalf("unfinished stakes not refunded exactly: %+v", restarted.profiles)
	}
}

func TestReviewSocialWebSocketIsolation(t *testing.T) {
	t.Setenv("TABLE_CARD_DATA_DIR", t.TempDir())
	s := New()
	h := httptest.NewServer(s.Handler())
	t.Cleanup(func() { s.Close(); h.Close() })
	type client struct {
		conn   *websocket.Conn
		events chan map[string]any
		id     string
	}
	connect := func(name string, mode table.Mode) client {
		id, token := testIdentity(strings.Repeat(name, 64))
		headers := http.Header{"X-Player-ID": {id}, "X-Player-Token": {token}}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.URL, "http")+"/api/social/ws?playerId="+id+"&name="+name+"&mode="+string(mode), &websocket.DialOptions{HTTPHeader: headers})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		c := client{conn: conn, events: make(chan map[string]any, 64), id: id}
		go func() {
			defer close(c.events)
			for {
				_, raw, err := conn.Read(context.Background())
				if err != nil {
					return
				}
				var e map[string]any
				if json.Unmarshal(raw, &e) == nil {
					c.events <- e
				}
			}
		}()
		return c
	}
	a, b, c := connect("A", table.GardenMode), connect("B", table.PokemonMode), connect("C", table.GoMode)
	wait := func(c client, text string, forbidden string) {
		t.Helper()
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		for {
			select {
			case e, ok := <-c.events:
				if !ok {
					t.Fatal("socket closed")
				}
				if forbidden != "" && e["text"] == forbidden {
					t.Fatal("private message leaked")
				}
				if e["text"] == text {
					return
				}
			case <-timer.C:
				t.Fatal("message timed out:", text)
			}
		}
	}
	send := func(c client, r socialRequest) {
		raw, _ := json.Marshal(r)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := c.conn.Write(ctx, websocket.MessageText, raw); err != nil {
			t.Fatal(err)
		}
	}
	send(a, socialRequest{Type: "chat", Scope: "server", Text: "cross-mode"})
	for _, p := range []client{a, b, c} {
		wait(p, "cross-mode", "")
	}
	send(b, socialRequest{Type: "chat", Scope: "direct", TargetID: a.id, Text: "private"})
	wait(a, "private", "")
	wait(b, "private", "")
	send(c, socialRequest{Type: "chat", Scope: "server", Text: "barrier"})
	wait(c, "barrier", "private")
}

func TestReviewSingleIdentityCannotOccupyMultipleRooms(t *testing.T) {
	s := reviewServer(t)
	id, token := testIdentity(strings.Repeat("multiroom", 8))
	post := func() int {
		body := `{"mode":"garden_relay","seats":2,"bots":1,"playerId":"` + id + `","name":"A"}`
		r := httptest.NewRequest("POST", "/api/rooms", strings.NewReader(body))
		r.Header.Set("X-Player-ID", id)
		r.Header.Set("X-Player-Token", token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w.Code
	}
	if post() != 201 {
		t.Fatal("first room creation failed")
	}
	if post() < 400 {
		t.Fatal("same identity accepted in a second room; presence and room-chat become ambiguous")
	}
}
