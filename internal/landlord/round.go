package landlord

import (
	"errors"
	"fmt"

	"github.com/17hwliao/table-card-independent/internal/cards"
)

type Phase uint8

const (
	Auction Phase = iota
	Playing
	Redeal
	Complete
)

var (
	ErrWrongPhase = errors.New("当前阶段不能执行该操作")
	ErrWrongSeat  = errors.New("尚未轮到该座位")
	ErrBadBid     = errors.New("叫分必须在 0 到 3 之间，并且高于当前叫分")
	ErrNotOwned   = errors.New("所选牌不全在你的手牌中")
	ErrMustLead   = errors.New("新一轮必须出牌，不能先过牌")
)

type Trick struct {
	Seat    int
	Cards   []cards.Card
	Pattern Hand
}

type Round struct {
	phase       Phase
	hands       [3][]cards.Card
	bottom      []cards.Card
	turn        int
	bidCount    int
	highestBid  int
	highestSeat int
	landlord    int
	trick       *Trick
	passes      int
	multiplier  int
	winner      int
}

func NewRound() *Round {
	deck := cards.NewDeck()
	cards.Shuffle(deck)
	round := &Round{
		phase:       Auction,
		highestSeat: -1,
		landlord:    -1,
		multiplier:  1,
		winner:      -1,
	}
	for seat := range round.hands {
		start := seat * 17
		round.hands[seat] = append([]cards.Card(nil), deck[start:start+17]...)
		cards.Sort(round.hands[seat])
	}
	round.bottom = append([]cards.Card(nil), deck[51:]...)
	return round
}

func (r *Round) Phase() Phase           { return r.phase }
func (r *Round) Turn() int              { return r.turn }
func (r *Round) Landlord() int          { return r.landlord }
func (r *Round) HighestBid() int        { return r.highestBid }
func (r *Round) Multiplier() int        { return r.multiplier }
func (r *Round) Winner() int            { return r.winner }
func (r *Round) BidCount() int          { return r.bidCount }
func (r *Round) CardsLeft(seat int) int { return len(r.hands[seat]) }

func (r *Round) Hand(seat int) []cards.Card {
	return append([]cards.Card(nil), r.hands[seat]...)
}

func (r *Round) Bottom() []cards.Card {
	if r.phase == Auction || r.phase == Redeal {
		return nil
	}
	return append([]cards.Card(nil), r.bottom...)
}

func (r *Round) CurrentTrick() *Trick {
	if r.trick == nil {
		return nil
	}
	copy := *r.trick
	copy.Cards = append([]cards.Card(nil), r.trick.Cards...)
	return &copy
}

func (r *Round) Bid(seat, value int) error {
	if r.phase != Auction {
		return ErrWrongPhase
	}
	if seat != r.turn {
		return ErrWrongSeat
	}
	if value < 0 || value > 3 || value != 0 && value <= r.highestBid {
		return ErrBadBid
	}
	if value > r.highestBid {
		r.highestBid = value
		r.highestSeat = seat
	}
	r.bidCount++
	if r.bidCount < 3 {
		r.turn = (seat + 1) % len(r.hands)
		return nil
	}
	if r.highestSeat < 0 {
		r.phase = Redeal
		return nil
	}
	r.landlord = r.highestSeat
	r.multiplier = r.highestBid
	r.hands[r.landlord] = append(r.hands[r.landlord], r.bottom...)
	cards.Sort(r.hands[r.landlord])
	r.turn = r.landlord
	r.phase = Playing
	return nil
}

// Play accepts an empty selection as a pass. A non-empty selection is checked
// against the player's own cards and, when following, against the current trick.
func (r *Round) Play(seat int, selected []cards.Card) error {
	if r.phase != Playing {
		return ErrWrongPhase
	}
	if seat != r.turn {
		return ErrWrongSeat
	}
	if len(selected) == 0 {
		if r.trick == nil || r.trick.Seat == seat {
			return ErrMustLead
		}
		r.passes++
		if r.passes == 2 {
			r.turn = r.trick.Seat
			r.trick = nil
			r.passes = 0
			return nil
		}
		r.turn = (seat + 1) % len(r.hands)
		return nil
	}

	pattern, err := Classify(selected)
	if err != nil {
		return err
	}
	if r.trick != nil && r.trick.Seat != seat && !Beats(pattern, r.trick.Pattern) {
		return fmt.Errorf("出牌不能压过当前牌型")
	}
	remaining, ok := takeCards(r.hands[seat], selected)
	if !ok {
		return ErrNotOwned
	}
	r.hands[seat] = remaining
	r.trick = &Trick{Seat: seat, Cards: append([]cards.Card(nil), selected...), Pattern: pattern}
	r.passes = 0
	if pattern.Kind == Bomb || pattern.Kind == Rocket {
		r.multiplier *= 2
	}
	if len(remaining) == 0 {
		r.phase = Complete
		r.winner = seat
		return nil
	}
	r.turn = (seat + 1) % len(r.hands)
	return nil
}

func takeCards(hand, selected []cards.Card) ([]cards.Card, bool) {
	used := make([]bool, len(hand))
	for _, wanted := range selected {
		found := false
		for i, held := range hand {
			if !used[i] && held == wanted {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	remaining := make([]cards.Card, 0, len(hand)-len(selected))
	for i, held := range hand {
		if !used[i] {
			remaining = append(remaining, held)
		}
	}
	return remaining, true
}
