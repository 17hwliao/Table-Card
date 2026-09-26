package mahjong

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/table"
)

func tiles(suit uint8, ranks ...uint8) []Tile {
	out := make([]Tile, len(ranks))
	for i, rank := range ranks {
		out[i] = Tile{suit, rank}
	}
	return out
}

func TestAdditivePatternsAndRoots(t *testing.T) {
	cases := []struct {
		name  string
		hand  []Tile
		melds []Meld
		want  int
	}{
		{"plain", append(tiles(0, 1, 2, 3, 4, 5, 6), tiles(1, 1, 2, 3, 7, 8, 9, 5, 5)...), nil, 1},
		{"pure", tiles(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 5, 5), nil, 4},
		{"triplets", append(tiles(0, 1, 1, 1, 2, 2, 2), tiles(1, 4, 4, 4, 7, 7, 7, 9, 9)...), nil, 2},
		{"seven pairs", append(tiles(0, 1, 1, 2, 2, 3, 3), tiles(1, 4, 4, 5, 5, 6, 6, 7, 7)...), nil, 4},
		{"dragon pairs plus root", append(tiles(0, 1, 1, 1, 1, 2, 2), tiles(1, 4, 4, 5, 5, 6, 6, 7, 7)...), nil, 7},
		{"pure dragon", tiles(0, 1, 1, 1, 1, 2, 2, 4, 4, 5, 5, 6, 6, 7, 7), nil, 11},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := scoreFan(tt.hand, tt.melds); got != tt.want {
				t.Fatalf("fan %d, want %d", got, tt.want)
			}
		})
	}
	melds := []Meld{}
	for rank := uint8(1); rank <= 4; rank++ {
		melds = append(melds, Meld{Kind: "peng", Tiles: tiles(0, rank, rank, rank)})
	}
	if got := scoreFan(tiles(0, 5, 5), melds); got != 8 {
		t.Fatalf("pure 4 + triplets 2 + golden hook 2 = 8, got %d", got)
	}
	if !sevenPairs(cases[4].hand) {
		t.Fatal("four identical tiles must count as two pairs")
	}
}

func fixture(t *testing.T) *Game {
	t.Helper()
	g, err := New([]table.Player{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}, {ID: "d", Name: "D"}})
	if err != nil {
		t.Fatal(err)
	}
	g.hands = [Players][]Tile{}
	g.melds = [Players][]Meld{}
	g.discards = [Players][]Tile{}
	g.missing = [Players]int{2, 2, 2, 2}
	g.wall = tiles(1, 9)
	g.dealer = 0
	return g
}

func TestReadyFanIgnoresDeadWaitAndAppliesCap(t *testing.T) {
	g := fixture(t)
	// Four concealed triplets leave only the pair wait at 9筒.
	g.hands[0] = append(tiles(0, 1, 1, 1, 2, 2, 2, 3, 3, 3), tiles(1, 4, 4, 4, 9)...)
	if got := g.readyFan(0); got != 2 {
		t.Fatalf("live wait fan=%d, want2", got)
	}
	g.discards[1] = tiles(1, 9, 9, 9)
	if got := g.readyFan(0); got != 0 {
		t.Fatalf("all 9筒 exposed, dead wait fan=%d", got)
	}
	g.discards[1] = nil
	g.optionsConfig.FanCap = 1
	if got := g.readyFan(0); got != 1 {
		t.Fatalf("cap not applied: %d", got)
	}
}

func TestSpecialFanAndZeroSumSettlement(t *testing.T) {
	g := fixture(t)
	g.hands[0] = append(tiles(0, 1, 1, 1, 2, 2, 2), tiles(1, 4, 4, 4, 7, 7, 7, 9, 9)...)
	g.phase = "turn"
	g.gangDraw = true
	g.wall = nil
	g.optionsConfig.SelfDrawBonus = true
	g.optionsConfig.BaseScore = 3
	// Triplets 2 + gang draw 1 + sea bottom 1 + optional self draw 1.
	g.settleWin(0, -1, true)
	if g.scores != [Players]int{45, -15, -15, -15} {
		t.Fatalf("scores=%v", g.scores)
	}
	g.optionsConfig.FanCap = 4
	if got := g.winFan(0, g.hands[0], true); got != 4 {
		t.Fatalf("final cap =%d", got)
	}
}

func TestFlowRefundsOriginalPayersAndChecksReady(t *testing.T) {
	g := fixture(t)
	g.active = [Players]bool{true, true, false, false}
	g.hands[0] = tiles(0, 1, 2, 4, 5, 7, 8, 9, 1, 3, 5, 7, 9, 9)
	g.hands[1] = append(tiles(0, 1, 1, 1, 2, 2, 2, 3, 3, 3), tiles(1, 4, 4, 4, 9)...)
	g.applyGangPayment(2, 0, 2)
	g.applyGangPayment(0, 1, 1)
	g.endRound()
	if g.scores != [Players]int{-3, 3, 0, 0} {
		t.Fatalf("refund then ready compensation: %v", g.scores)
	}
}

func TestSeaDiscardCannotPeng(t *testing.T) {
	g := fixture(t)
	g.wall = nil
	g.hands[1] = tiles(0, 1, 1)
	g.pending = &Tile{0, 1}
	g.discarder = 0
	g.startClaims(0, *g.pending)
	if !g.finished {
		t.Fatal("sea discard without hu must immediately flow")
	}
}

func TestBotCompletesRoundsWithLegalActions(t *testing.T) {
	for round := 0; round < 8; round++ {
		g := fixture(t)
		g.resetHand()
		e := &Engine{game: g}
		for step := 0; step < 1500 && !g.finished; step++ {
			acted := false
			for _, p := range g.players {
				raw := e.BotAction(p.ID)
				if raw == nil {
					continue
				}
				if !json.Valid(raw) {
					t.Fatal("invalid bot JSON")
				}
				if _, err := e.Apply(p.ID, raw); err != nil {
					t.Fatalf("round%d step%d player%s action%s: %v", round, step, p.ID, raw, err)
				}
				acted = true
				break
			}
			if !acted {
				t.Fatalf("bots stalled in %s", g.phase)
			}
		}
		if !g.finished {
			t.Fatal("round did not finish")
		}
		sum := 0
		for _, score := range g.scores {
			sum += score
		}
		if sum != 0 {
			t.Fatalf("score not conserved: %v", g.scores)
		}
		if e.BotAction("a") != nil {
			t.Fatal("bot must not automatically restart finished round")
		}
	}
}

func TestOptionsDisableExchange(t *testing.T) {
	opts := DefaultOptions()
	opts.ExchangeThree = false
	e, err := NewEngineWithOptions([]table.Player{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if e.game.phase != "missing" {
		t.Fatalf("phase=%s", e.game.phase)
	}
}

func TestInvalidActionsPreserveHandsAndState(t *testing.T) {
	for _, action := range []string{`{"type":"discard","suit":0,"rank":1}`, `{"type":"add_gang","suit":0,"rank":1}`, `{"type":"gang","suit":0,"rank":1}`} {
		g := fixture(t)
		g.phase = "turn"
		g.turn = 0
		g.hasDrawn = true
		g.hands[0] = []Tile{{0, 1}, {2, 5}, {0, 2}, {0, 1}}
		before := append([]Tile(nil), g.hands[0]...)
		beforeView, _ := json.Marshal(g.View("a"))
		if _, err := g.Apply("a", json.RawMessage(action)); err == nil {
			t.Fatalf("expected action rejection: %s", action)
		}
		afterView, _ := json.Marshal(g.View("a"))
		if !reflect.DeepEqual(before, g.hands[0]) || string(beforeView) != string(afterView) {
			t.Fatalf("rejected action changed state: %s", action)
		}
	}
}

func TestReadyFanIncludesMaximumDragonPattern(t *testing.T) {
	g := fixture(t)
	g.hands[0] = tiles(0, 1, 1, 1, 2, 2, 4, 4, 5, 5, 6, 6, 7, 7)
	if got := g.readyFan(0); got != 11 {
		t.Fatalf("maximum pure dragon + root =11, got%d", got)
	}
	g.optionsConfig.FanCap = 8
	if got := g.readyFan(0); got != 8 {
		t.Fatalf("ready cap=8, got%d", got)
	}
}

func TestSimultaneousWinsOverridePengAndPreserveFirstDealer(t *testing.T) {
	g := fixture(t)
	g.phase = "turn"
	g.turn = 0
	g.hasDrawn = true
	g.hands[0] = tiles(1, 9)
	g.hands[1] = append(tiles(0, 1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4), Tile{1, 9})
	g.hands[2] = append(tiles(0, 5, 5, 5, 6, 6, 6, 7, 7, 7, 8, 8, 8), Tile{1, 9})
	// Player3 cannot have two 9筒 here (five copies); use a legal pass.
	g.hands[3] = tiles(1, 1, 2, 3)
	if _, err := g.Apply("a", json.RawMessage(`{"type":"discard","suit":1,"rank":9}`)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"b", "c"} {
		if _, err := g.Apply(id, json.RawMessage(`{"type":"claim","claim":"hu"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.winners) != 2 || g.active[1] || g.active[2] {
		t.Fatalf("simultaneous winners: %+v", g.winners)
	}
	if g.nextDealer != 1 {
		t.Fatalf("nearest first winner should be next dealer, got%d", g.nextDealer)
	}
	if g.scores != [Players]int{-4, 2, 2, 0} {
		t.Fatalf("point-source pays both winners: %v", g.scores)
	}
}

func TestBotSelectionDoesNotMutateOrInspectOpponentsHands(t *testing.T) {
	g := fixture(t)
	g.phase = "turn"
	g.turn = 0
	g.hasDrawn = true
	g.hands[0] = tiles(0, 1, 2, 3, 5, 5, 8)
	e := &Engine{game: g}
	before := append([]Tile(nil), g.hands[0]...)
	first := e.BotAction("a")
	g.hands[1] = tiles(0, 1, 1, 1, 2, 2, 2, 3, 3, 3)
	second := e.BotAction("a")
	if string(first) != string(second) || !reflect.DeepEqual(before, g.hands[0]) {
		t.Fatal("bot decision must use own hand and must be read-only")
	}
}

func TestViewHasIndependentMeldsAndSimultaneousMissing(t *testing.T) {
	g := fixture(t)
	g.phase = "missing"
	g.ready[0] = true
	g.melds[0] = []Meld{{Kind: "peng", Tiles: tiles(0, 1, 1, 1)}}
	if view := g.View("b"); view.Players[0].MissingSuit != -1 {
		t.Fatal("other players must not see missing selection before everyone chooses")
	}
	view := g.View("a")
	view.Players[0].Melds[0].Tiles[0] = Tile{2, 9}
	if g.melds[0][0].Tiles[0] != (Tile{0, 1}) {
		t.Fatal("returned snapshot aliases live meld tiles")
	}
}
