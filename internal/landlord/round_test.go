package landlord_test

import (
	"reflect"
	"testing"

	"github.com/17hwliao/table-card-independent/internal/bot/sunjiajia"
	"github.com/17hwliao/table-card-independent/internal/cards"
	"github.com/17hwliao/table-card-independent/internal/landlord"
)

func hand(ranks ...cards.Rank) []cards.Card {
	counts := map[cards.Rank]int{}
	result := make([]cards.Card, 0, len(ranks))
	for _, rank := range ranks {
		suit := cards.Suit(counts[rank])
		if rank >= cards.SmallJoker {
			suit = cards.Joker
		}
		result = append(result, cards.Card{Suit: suit, Rank: rank})
		counts[rank]++
	}
	return result
}

func TestEveryLandlordPatternAndComparison(t *testing.T) {
	cases := []struct {
		kind  landlord.Kind
		ranks []cards.Rank
	}{
		{landlord.Pass, nil},
		{landlord.Single, []cards.Rank{3}},
		{landlord.Pair, []cards.Rank{3, 3}},
		{landlord.Triple, []cards.Rank{3, 3, 3}},
		{landlord.TripleSingle, []cards.Rank{3, 3, 3, 4}},
		{landlord.TriplePair, []cards.Rank{3, 3, 3, 4, 4}},
		{landlord.Straight, []cards.Rank{3, 4, 5, 6, 7}},
		{landlord.ConsecutivePairs, []cards.Rank{3, 3, 4, 4, 5, 5}},
		{landlord.Airplane, []cards.Rank{3, 3, 3, 4, 4, 4}},
		{landlord.AirplaneSingles, []cards.Rank{3, 3, 3, 4, 4, 4, 7, 8}},
		{landlord.AirplanePairs, []cards.Rank{3, 3, 3, 4, 4, 4, 7, 7, 8, 8}},
		{landlord.FourWithSingles, []cards.Rank{3, 3, 3, 3, 7, 8}},
		{landlord.FourWithPairs, []cards.Rank{3, 3, 3, 3, 7, 7, 8, 8}},
		{landlord.Bomb, []cards.Rank{3, 3, 3, 3}},
		{landlord.Rocket, []cards.Rank{cards.SmallJoker, cards.BigJoker}},
	}
	for _, tc := range cases {
		got, err := landlord.Classify(hand(tc.ranks...))
		if err != nil || got.Kind != tc.kind {
			t.Fatalf("pattern %d classified as %+v: %v", tc.kind, got, err)
		}
		if landlord.Beats(got, got) {
			t.Fatal("equal patterns cannot beat one another")
		}
	}
	for _, ranks := range [][]cards.Rank{{3, 4}, {3, 3, 4}, {10, 11, 12, 13, 14, 15}, {3, 4, 5, 7, 8}} {
		if _, err := landlord.Classify(hand(ranks...)); err == nil {
			t.Fatalf("invalid hand accepted: %v", ranks)
		}
	}
	single, _ := landlord.Classify(hand(3))
	pair, _ := landlord.Classify(hand(4, 4))
	bomb, _ := landlord.Classify(hand(3, 3, 3, 3))
	rocket, _ := landlord.Classify(hand(cards.SmallJoker, cards.BigJoker))
	if landlord.Beats(pair, single) || !landlord.Beats(bomb, pair) || !landlord.Beats(rocket, bomb) || landlord.Beats(bomb, rocket) {
		t.Fatal("pattern priority changed")
	}
}

func TestAuctionRedealOwnershipAndPassing(t *testing.T) {
	r := landlord.NewRound()
	if r.Bottom() != nil || r.Play(0, nil) != landlord.ErrWrongPhase || r.Bid(1, 1) != landlord.ErrWrongSeat || r.Bid(0, 4) != landlord.ErrBadBid {
		t.Fatal("auction validation failed")
	}
	for seat := range 3 {
		if r.CardsLeft(seat) != 17 || r.Bid(seat, 0) != nil {
			t.Fatal("initial deal or pass bid failed")
		}
	}
	if r.Phase() != landlord.Redeal {
		t.Fatal("three zero bids did not request redeal")
	}
	r = landlord.NewRound()
	r.Bid(0, 1)
	if r.Bid(1, 1) != landlord.ErrBadBid {
		t.Fatal("equal bid accepted")
	}
	r.Bid(1, 0)
	r.Bid(2, 0)
	if r.Phase() != landlord.Playing || r.CardsLeft(0) != 20 || len(r.Bottom()) != 3 || r.Landlord() != 0 || r.HighestBid() != 1 || r.BidCount() != 3 || r.Multiplier() != 1 {
		t.Fatal("landlord settlement failed")
	}
	if r.Play(0, nil) != landlord.ErrMustLead || r.Play(1, nil) != landlord.ErrWrongSeat || r.Bid(0, 2) != landlord.ErrWrongPhase {
		t.Fatal("play turn validation failed")
	}
	before := r.Hand(0)
	if r.Play(0, r.Hand(1)[:1]) != landlord.ErrNotOwned || !reflect.DeepEqual(before, r.Hand(0)) {
		t.Fatal("illegal card changed the hand")
	}
	owned := r.Hand(0)
	if err := r.Play(0, owned[:1]); err != nil {
		t.Fatal(err)
	}
	trick := r.CurrentTrick()
	trick.Cards[0].Rank = 0
	if r.CurrentTrick().Cards[0].Rank == 0 {
		t.Fatal("trick snapshot aliases game state")
	}
	r.Play(1, nil)
	r.Play(2, nil)
	if r.Turn() != 0 || r.CurrentTrick() != nil {
		t.Fatal("two passes did not reset the trick")
	}
}

func TestThirtyBotGamesFinishWithLegalOwnedCards(t *testing.T) {
	a := sunjiajia.Agent{}
	for range 30 {
		r := landlord.NewRound()
		r.Bid(0, 1)
		r.Bid(1, 0)
		r.Bid(2, 0)
		for step := 0; r.Phase() == landlord.Playing && step < 200; step++ {
			seat := r.Turn()
			view := sunjiajia.Table{Seat: seat, Landlord: r.Landlord(), LastTrick: r.CurrentTrick()}
			for i := range 3 {
				view.CardsLeft[i] = r.CardsLeft(i)
			}
			before := r.Hand(seat)
			play := a.Choose(before, view)
			if !reflect.DeepEqual(before, r.Hand(seat)) {
				t.Fatal("bot mutated its hand snapshot")
			}
			if err := r.Play(seat, play); err != nil {
				t.Fatalf("bot chose an illegal action: %v", err)
			}
			if r.CardsLeft(seat) != len(before)-len(play) {
				t.Fatal("cards not removed exactly once")
			}
		}
		if r.Phase() != landlord.Complete || r.CardsLeft(r.Winner()) != 0 {
			t.Fatal("bot game failed to finish")
		}
	}
}
