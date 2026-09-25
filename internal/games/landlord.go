package games

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/cards"
	"github.com/17hwliao/table-card-independent/internal/landlord"
	"github.com/17hwliao/table-card-independent/internal/table"
)

type landlordEngine struct {
	mu      sync.Mutex
	players []table.Player
	round   *landlord.Round
}

type landlordView struct {
	Phase      landlord.Phase  `json:"phase"`
	TurnSeat   int             `json:"turnSeat"`
	TurnPlayer string          `json:"turnPlayer"`
	Landlord   int             `json:"landlord"`
	HighestBid int             `json:"highestBid"`
	Multiplier int             `json:"multiplier"`
	Winner     int             `json:"winner"`
	CardsLeft  []int           `json:"cardsLeft"`
	Hand       []cards.Card    `json:"hand,omitempty"`
	Bottom     []cards.Card    `json:"bottom,omitempty"`
	Trick      *landlord.Trick `json:"trick,omitempty"`
}

func newLandlordEngine(players []table.Player) (Engine, error) {
	if len(players) != 3 {
		return nil, errors.New("斗地主需要 3 名玩家")
	}
	return &landlordEngine{players: append([]table.Player(nil), players...), round: landlord.NewRound()}, nil
}

func (e *landlordEngine) Mode() table.Mode { return table.LandlordMode }

func (e *landlordEngine) View(viewerID string) any {
	e.mu.Lock()
	defer e.mu.Unlock()
	view := landlordView{
		Phase: e.round.Phase(), TurnSeat: e.round.Turn(), Landlord: e.round.Landlord(),
		HighestBid: e.round.HighestBid(), Multiplier: e.round.Multiplier(), Winner: e.round.Winner(),
		Bottom: e.round.Bottom(), Trick: e.round.CurrentTrick(),
		CardsLeft: make([]int, len(e.players)),
	}
	if view.TurnSeat >= 0 && view.TurnSeat < len(e.players) {
		view.TurnPlayer = e.players[view.TurnSeat].ID
	}
	for seat := range e.players {
		view.CardsLeft[seat] = e.round.CardsLeft(seat)
		if e.players[seat].ID == viewerID {
			view.Hand = e.round.Hand(seat)
		}
	}
	return view
}

func (e *landlordEngine) Apply(playerID string, payload json.RawMessage) (any, error) {
	var action struct {
		Type  string       `json:"type"`
		Bid   int          `json:"bid"`
		Cards []cards.Card `json:"cards"`
	}
	if err := decodeAction(payload, &action); err != nil {
		return nil, err
	}
	seat := -1
	for index, player := range e.players {
		if player.ID == playerID {
			seat = index
			break
		}
	}
	if seat < 0 {
		return nil, errors.New("玩家不在本局中")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	switch action.Type {
	case "bid":
		if err := e.round.Bid(seat, action.Bid); err != nil {
			return nil, err
		}
		if e.round.Phase() == landlord.Redeal {
			e.round = landlord.NewRound()
		}
	case "play":
		if err := e.round.Play(seat, action.Cards); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("斗地主操作类型必须是 bid 或 play")
	}
	return e.viewLocked(playerID), nil
}

func (e *landlordEngine) viewLocked(viewerID string) landlordView {
	view := landlordView{
		Phase: e.round.Phase(), TurnSeat: e.round.Turn(), Landlord: e.round.Landlord(),
		HighestBid: e.round.HighestBid(), Multiplier: e.round.Multiplier(), Winner: e.round.Winner(),
		Bottom: e.round.Bottom(), Trick: e.round.CurrentTrick(),
		CardsLeft: make([]int, len(e.players)),
	}
	if view.TurnSeat >= 0 && view.TurnSeat < len(e.players) {
		view.TurnPlayer = e.players[view.TurnSeat].ID
	}
	for seat := range e.players {
		view.CardsLeft[seat] = e.round.CardsLeft(seat)
		if e.players[seat].ID == viewerID {
			view.Hand = e.round.Hand(seat)
		}
	}
	return view
}
