package mahjong

import "encoding/json"

// BotAction reads only the requesting seat's hand and public state. The room
// scheduler applies the returned action after its normal thinking interval.
func (e *Engine) BotAction(playerID string) json.RawMessage {
	g := e.game
	g.mu.RLock()
	defer g.mu.RUnlock()
	seat := -1
	for i, p := range g.players {
		if p.ID == playerID {
			seat = i
			break
		}
	}
	if seat < 0 || g.finished || !g.active[seat] {
		return nil
	}
	action := func(v any) json.RawMessage { raw, _ := json.Marshal(v); return raw }
	tileAction := func(kind string, tile Tile) json.RawMessage {
		return action(map[string]any{"type": kind, "suit": tile.Suit, "rank": tile.Rank})
	}
	hand := g.hands[seat]
	switch g.phase {
	case "exchange":
		if g.exchangeChosen[seat] {
			return nil
		}
		var suits [3][]Tile
		for _, tile := range hand {
			suits[tile.Suit] = append(suits[tile.Suit], tile)
		}
		chosen := -1
		for i := range suits {
			if len(suits[i]) >= 3 && (chosen < 0 || len(suits[i]) < len(suits[chosen])) {
				chosen = i
			}
		}
		if chosen >= 0 {
			return action(map[string]any{"type": "exchange", "tiles": suits[chosen][:3]})
		}
	case "missing":
		if g.ready[seat] {
			return nil
		}
		var counts [3]int
		for _, tile := range hand {
			counts[tile.Suit]++
		}
		chosen := 0
		for i := 1; i < 3; i++ {
			if counts[i] < counts[chosen] {
				chosen = i
			}
		}
		return action(map[string]any{"type": "missing", "suit": chosen})
	case "claim", "rob_gang":
		if seat == g.discarder || g.responses[seat] != "" {
			return nil
		}
		for _, claim := range []string{"hu", "gang", "peng"} {
			if containsString(g.options[seat], claim) {
				return action(map[string]string{"type": "claim", "claim": claim})
			}
		}
		return action(map[string]string{"type": "claim", "claim": "pass"})
	case "turn":
		if g.turn != seat {
			return nil
		}
		if !g.hasDrawn {
			return action(map[string]string{"type": "draw"})
		}
		if g.canSelfWin(seat) {
			return action(map[string]string{"type": "hu"})
		}
		if len(g.wall) > 0 {
			for _, tile := range hand {
				if int(tile.Suit) == g.missing[seat] {
					continue
				}
				if countTile(hand, tile) == 4 {
					return tileAction("gang", tile)
				}
				for _, meld := range g.melds[seat] {
					if meld.Kind == "peng" && len(meld.Tiles) == 3 && meld.Tiles[0] == tile {
						return tileAction("add_gang", tile)
					}
				}
			}
		}
		for _, tile := range hand {
			if int(tile.Suit) == g.missing[seat] {
				return tileAction("discard", tile)
			}
		}
		// Keep pairs/triplets and neighbouring tiles. Prefer terminal isolated
		// discards, with stable tie breaking so decisions are reproducible.
		best, bestValue := -1, int(^uint(0)>>1)
		for i, tile := range hand {
			value := (countTile(hand, tile) - 1) * 6
			for _, other := range hand {
				if other.Suit == tile.Suit {
					d := int(other.Rank) - int(tile.Rank)
					if d == 1 || d == -1 {
						value += 2
					}
					if d == 2 || d == -2 {
						value++
					}
				}
			}
			if tile.Rank == 1 || tile.Rank == 9 {
				value--
			}
			if value < bestValue {
				best, bestValue = i, value
			}
		}
		if best >= 0 {
			return tileAction("discard", hand[best])
		}
	}
	return nil
}
