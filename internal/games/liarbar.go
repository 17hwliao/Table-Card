package games

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand/v2"

	"github.com/17hwliao/table-card-independent/internal/liarbar"
	"github.com/17hwliao/table-card-independent/internal/table"
)

type liarBarEngine struct {
	players []table.Player
	game    *liarbar.Game
}

func newLiarBarEngine(players []table.Player) (Engine, error) {
	barPlayers := make([]liarbar.Player, len(players))
	for i, player := range players {
		barPlayers[i] = liarbar.Player{ID: player.ID, Name: player.Name}
	}
	game, err := liarbar.NewGame(barPlayers)
	if err != nil {
		return nil, err
	}
	return &liarBarEngine{players: append([]table.Player(nil), players...), game: game}, nil
}

func (e *liarBarEngine) Mode() table.Mode { return table.LiarBarMode }

func (e *liarBarEngine) View(viewerID string) any {
	return e.game.Snapshot(viewerID)
}

func (e *liarBarEngine) Apply(playerID string, payload json.RawMessage) (any, error) {
	var action struct {
		Type    string  `json:"type"`
		CardIDs []uint8 `json:"cardIds"`
	}
	if err := decodeAction(payload, &action); err != nil {
		return nil, err
	}
	result := struct {
		Snapshot  liarbar.Snapshot         `json:"snapshot"`
		Challenge *liarbar.ChallengeResult `json:"challenge,omitempty"`
	}{}
	switch action.Type {
	case "play":
		if err := e.game.Play(playerID, action.CardIDs); err != nil {
			return nil, err
		}
	case "challenge":
		challenge, err := e.game.Challenge(playerID)
		if err != nil {
			return nil, err
		}
		result.Challenge = &challenge
	default:
		return nil, errors.New("骗子酒馆操作类型必须是 play 或 challenge")
	}
	result.Snapshot = e.game.Snapshot(playerID)
	return result, nil
}

func decodeAction(payload json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

// BotAction uses only this player's hand and public declarations, never hidden cards.
func (e *liarBarEngine) BotAction(playerID string) json.RawMessage {
	v := e.game.Snapshot(playerID)
	if v.Phase != liarbar.RoundActive || v.Turn < 0 || v.Turn >= len(v.Players) || v.Players[v.Turn].ID != playerID {
		return nil
	}
	var action any
	honest := []uint8{}
	for _, card := range v.Hand {
		if card.Rank == v.Target || card.Rank == liarbar.Joker {
			honest = append(honest, card.ID)
		}
	}
	if v.Pending != nil && (len(v.Hand) == 0 || (len(honest) == 0 && rand.IntN(2) == 0) || (v.Pending.Count == 3 && rand.IntN(3) == 0)) {
		action = map[string]any{"type": "challenge"}
	} else {
		ids := honest
		if len(ids) == 0 && len(v.Hand) > 0 {
			ids = []uint8{v.Hand[0].ID}
		}
		if len(ids) > 2 {
			ids = ids[:2]
		}
		if len(ids) == 0 {
			return nil
		}
		action = map[string]any{"type": "play", "cardIds": ids}
	}
	data, _ := json.Marshal(action)
	return data
}
