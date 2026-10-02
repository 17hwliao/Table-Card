package cards

import "testing"

func TestDeckShuffleAndOrdering(t *testing.T) {
	deck := NewDeck()
	seen := map[Card]bool{}
	for _, c := range deck {
		if seen[c] || c.String() == "" {
			t.Fatal("duplicate or unnamed card")
		}
		seen[c] = true
	}
	if len(seen) != 54 {
		t.Fatal("deck is not 54 unique cards")
	}
	Shuffle(deck)
	for _, c := range deck {
		if !seen[c] {
			t.Fatal("shuffle introduced a card")
		}
		delete(seen, c)
	}
	if len(seen) != 0 {
		t.Fatal("shuffle lost a card")
	}
	Sort(deck)
	for i := 1; i < len(deck); i++ {
		if deck[i-1].Rank < deck[i].Rank || (deck[i-1].Rank == deck[i].Rank && deck[i-1].Suit > deck[i].Suit) {
			t.Fatal("card ordering changed")
		}
	}
	if Rank(0).String() != "?" || Suit(99).String() != "" {
		t.Fatal("invalid card label changed")
	}
}
