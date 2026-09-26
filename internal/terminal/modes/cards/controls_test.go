package cards

import (
	"encoding/json"
	"testing"

	gamecards "github.com/17hwliao/table-card-independent/internal/cards"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

func TestLandlordCanSelectAndPlayTwentiethCard(t *testing.T) {
	hand := gamecards.NewDeck()[:20]
	data, _ := json.Marshal(landlordView{Phase: 1, TurnPlayer: "a", Hand: hand})
	s := ui.Snapshot{PlayerID: "a", Game: data}
	c := NewLandlord()
	c.Key(s, "end")
	c.Key(s, "space")
	r := c.Key(s, "enter")
	var a struct {
		Cards []gamecards.Card `json:"cards"`
	}
	if err := json.Unmarshal(r.Action, &a); err != nil {
		t.Fatal(err)
	}
	if len(a.Cards) != 1 || a.Cards[0] != hand[19] {
		t.Fatalf("20th card unreachable: %s", r.Action)
	}
}

func TestLandlordEscapeCancelsWithoutAction(t *testing.T) {
	data, _ := json.Marshal(landlordView{Phase: 1, TurnPlayer: "a", Hand: gamecards.NewDeck()[:20]})
	s := ui.Snapshot{PlayerID: "a", Game: data}
	c := NewLandlord()
	c.Key(s, "end")
	c.Key(s, "space")
	if r := c.Key(s, "esc"); r.Exit || len(r.Action) > 0 || len(c.selected) > 0 {
		t.Fatal("Escape must only cancel selection")
	}
}

func TestUNOCanJumpOutOfTurnFromKeyboard(t *testing.T) {
	data := json.RawMessage(`{"turnPlayer":"b","options":{"blitz":true},"hand":[{"id":12,"color":1,"kind":0,"number":5}]}`)
	c := NewUNO()
	s := ui.Snapshot{PlayerID: "a", Game: data}
	c.Key(s, "right")
	if action := c.Key(s, "enter").Action; len(action) == 0 {
		t.Fatal("UI prevented out-of-turn matching-card jump")
	}
}
