package cards

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

type Suit uint8
type Rank uint8

const (
	Spade Suit = iota
	Heart
	Club
	Diamond
	Joker
)

const (
	Three Rank = 3 + iota
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
	Two
	SmallJoker
	BigJoker
)

type Card struct {
	Suit Suit
	Rank Rank
}

func NewDeck() []Card {
	deck := make([]Card, 0, 54)
	for suit := Spade; suit <= Diamond; suit++ {
		for rank := Three; rank <= Two; rank++ {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	deck = append(deck,
		Card{Suit: Joker, Rank: SmallJoker},
		Card{Suit: Joker, Rank: BigJoker},
	)
	return deck
}

func Shuffle(deck []Card) {
	rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

func Sort(hand []Card) {
	slices.SortFunc(hand, func(a, b Card) int {
		if a.Rank != b.Rank {
			return int(b.Rank) - int(a.Rank)
		}
		return int(a.Suit) - int(b.Suit)
	})
}

func (r Rank) String() string {
	switch r {
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	case Two:
		return "2"
	case SmallJoker:
		return "小王"
	case BigJoker:
		return "大王"
	default:
		if r >= Three && r <= Ten {
			return fmt.Sprint(uint8(r))
		}
		return "?"
	}
}

func (s Suit) String() string {
	switch s {
	case Spade:
		return "♠"
	case Heart:
		return "♥"
	case Club:
		return "♣"
	case Diamond:
		return "♦"
	default:
		return ""
	}
}

func (c Card) String() string {
	if c.Suit == Joker {
		return c.Rank.String()
	}
	return c.Suit.String() + c.Rank.String()
}
