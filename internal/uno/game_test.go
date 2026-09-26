package uno

import (
	"encoding/json"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/table"
)

func fixture() *Game {
	g := &Game{options: DefaultOptions(), players: []table.Player{{ID: "a"}, {ID: "b"}, {ID: "c"}}, hands: make([][]Card, 3), scores: make([]int, 3), saidUNO: make([]bool, 3), direction: 1, color: Red, missedUNO: -1, openingSkip: -1}
	g.discard = []Card{{ID: 1000, Color: Red, Number: 5}}
	for i := 0; i < 30; i++ {
		g.draw = append(g.draw, Card{ID: 100 + i, Color: Blue, Number: 1})
	}
	return g
}

func play(t *testing.T, g *Game, player string, action any) Snapshot {
	t.Helper()
	data, _ := json.Marshal(action)
	s, err := g.Apply(player, data)
	if err != nil {
		t.Fatalf("%s action %s: %v", player, data, err)
	}
	return s
}

func TestZeroPassesHandsInCurrentDirection(t *testing.T) {
	for _, direction := range []int{1, -1} {
		g := fixture()
		g.direction = direction
		g.hands = [][]Card{{{ID: 1, Color: Red, Number: 0}, {ID: 2, Color: Blue, Number: 2}}, {{ID: 3, Color: Yellow, Number: 3}}, {{ID: 4, Color: Green, Number: 4}}}
		play(t, g, "a", map[string]any{"type": "play", "cardId": 1})
		if got := g.hands[g.next(0)][0].ID; got != 2 {
			t.Fatalf("direction %d: player's remaining card passed to wrong seat: %d", direction, got)
		}
		if g.hands[g.next(1)][0].ID != 3 || g.hands[g.next(2)][0].ID != 4 {
			t.Fatal("all hands must pass forward exactly once")
		}
	}
}

func TestBlitzOutOfTurnMovesTurnFromJumper(t *testing.T) {
	g := fixture()
	g.turn = 1
	g.hands = [][]Card{{{ID: 1, Color: Blue, Number: 9}}, {{ID: 2, Color: Yellow, Number: 2}}, {{ID: 3, Color: Red, Number: 5}, {ID: 4, Color: Green, Number: 6}}}
	play(t, g, "c", map[string]any{"type": "play", "cardId": 3, "uno": true})
	if g.turn != 0 || g.discard[len(g.discard)-1].ID != 3 {
		t.Fatal("jumping seat must become the new turn origin")
	}
}

func TestBlitzRejectsMismatchedCardWithoutMutating(t *testing.T) {
	g := fixture()
	g.turn = 1
	g.hands[2] = []Card{{ID: 3, Color: Blue, Number: 5}}
	_, err := g.Apply("c", json.RawMessage(`{"type":"play","cardId":3}`))
	if err == nil || g.turn != 1 || len(g.hands[2]) != 1 {
		t.Fatal("invalid jump changed state")
	}
}

func TestFinalPenaltyIncludesPreviousStackBeforeScoring(t *testing.T) {
	g := fixture()
	g.penalty = 6
	g.penaltyKind = WildDrawFour
	g.hands = [][]Card{{{ID: 1, Color: Red, Kind: DrawTwo}}, {{ID: 2, Color: Blue, Number: 2}}, {{ID: 3, Color: Green, Number: 3}}}
	play(t, g, "a", map[string]any{"type": "play", "cardId": 1})
	if !g.finished || len(g.hands[1]) != 9 || g.penalty != 0 {
		t.Fatalf("final stack not paid: hand=%d penalty=%d", len(g.hands[1]), g.penalty)
	}
	if g.scores[0] != 13 {
		t.Fatalf("score must include all final penalty cards, got %d", g.scores[0])
	}
}

func TestSuccessfulChallengeOnlyCancelsLatestFour(t *testing.T) {
	g := fixture()
	g.penalty = 2
	g.penaltyKind = DrawTwo
	g.hands = [][]Card{{{ID: 1, Kind: WildDrawFour}, {ID: 2, Color: Red, Number: 2}}, {{ID: 3, Color: Green, Number: 3}}, {{ID: 4, Color: Yellow, Number: 4}}}
	play(t, g, "a", map[string]any{"type": "play", "cardId": 1, "color": Green, "uno": true})
	play(t, g, "b", map[string]any{"type": "challenge"})
	if g.penalty != 2 || g.penaltyKind != DrawTwo || g.turn != 1 || len(g.hands[0]) != 5 || g.color != Red {
		t.Fatalf("prior stack/challenger turn lost: penalty=%d turn=%d", g.penalty, g.turn)
	}
}

func TestDoublePlayEndsRoundWithoutSevenExchange(t *testing.T) {
	g := fixture()
	g.options.DoublePlay = true
	g.hands = [][]Card{{{ID: 1, Color: Red, Number: 7}, {ID: 2, Color: Red, Number: 7}}, {{ID: 3, Color: Blue, Number: 3}}, {{ID: 4, Color: Yellow, Number: 4}}}
	play(t, g, "a", map[string]any{"type": "play", "cardId": 1, "pair": true, "target": -1})
	if !g.finished || g.winner != "a" || len(g.hands[1]) != 1 {
		t.Fatal("last seven pair must win without hand exchange")
	}
}

func TestOpeningWildRequiresFirstPlayerColor(t *testing.T) {
	g := fixture()
	g.openingColor = true
	g.color = NoColor
	if _, err := g.Apply("b", json.RawMessage(`{"type":"choose_color","color":2}`)); err == nil {
		t.Fatal("other player chose opening color")
	}
	play(t, g, "a", map[string]any{"type": "choose_color", "color": Yellow})
	if g.openingColor || g.color != Yellow || g.turn != 0 {
		t.Fatal("opening color should retain first turn")
	}
}

func TestBotsProduceLegalActionsAndConserveDeck(t *testing.T) {
	players := []table.Player{{ID: "a", Name: "A", Bot: true}, {ID: "b", Name: "B", Bot: true}, {ID: "c", Name: "C", Bot: true}, {ID: "d", Name: "D", Bot: true}}
	for round := 0; round < 4; round++ {
		e, err := NewEngine(players)
		if err != nil {
			t.Fatal(err)
		}
		for turn := 0; turn < 2000 && !e.game.finished; turn++ {
			id := players[e.game.turn].ID
			a := e.BotAction(id)
			if len(a) == 0 {
				t.Fatalf("bot stalled at turn %d", turn)
			}
			if _, err := e.Apply(id, a); err != nil {
				t.Fatalf("bot produced illegal %s: %v", a, err)
			}
			seen := map[int]bool{}
			check := func(cards []Card) {
				for _, c := range cards {
					if seen[c.ID] {
						t.Fatalf("duplicate card %d", c.ID)
					}
					seen[c.ID] = true
				}
			}
			check(e.game.draw)
			check(e.game.discard)
			for _, h := range e.game.hands {
				check(h)
			}
			if len(seen) != 108 {
				t.Fatalf("deck lost cards: %d", len(seen))
			}
		}
		if !e.game.finished {
			t.Fatal("bot game failed to finish")
		}
	}
}
