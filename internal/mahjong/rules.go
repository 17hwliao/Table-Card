package mahjong

import "errors"

// Options are agreed before dealing and remain fixed throughout the table.
type Options struct {
	ExchangeThree      bool `json:"exchangeThree"`
	BaseScore          int  `json:"baseScore"`
	FanCap             int  `json:"fanCap"`
	FlowerPigPenalty   int  `json:"flowerPigPenalty"`
	DragonPairBonus    int  `json:"dragonPairBonus"`
	SelfDrawBonus      bool `json:"selfDrawBonus"`
	HeavenlyEarthlyCap bool `json:"heavenlyEarthlyCap"`
}

func DefaultOptions() Options {
	return Options{ExchangeThree: true, BaseScore: 1, FanCap: 16, FlowerPigPenalty: 2, DragonPairBonus: 2}
}

func (o Options) Validate() error {
	if o.BaseScore < 1 || o.BaseScore > 10000 || o.FanCap < 0 || o.FanCap > 1000 || o.FlowerPigPenalty < 1 || o.FlowerPigPenalty > 100 || o.DragonPairBonus < 0 || o.DragonPairBonus > 100 {
		return errors.New("麻将房规超出范围：底分1–10000、封顶0–1000、花猪1–100、龙七对加番0–100")
	}
	if o.HeavenlyEarthlyCap && o.FanCap == 0 {
		return errors.New("天地胡满番需要设置封顶番数")
	}
	return nil
}

// Pattern values add; plain-hand's one fan applies only without another base
// pattern. A quad in seven pairs receives both the agreed dragon bonus and root.
func scoreFan(hand []Tile, melds []Meld) int { return patternFan(hand, melds, 2) }
func patternFan(hand []Tile, melds []Meld, dragonBonus int) int {
	if len(hand) == 0 {
		return 0
	}
	counts := map[Tile]int{}
	pure := true
	first := hand[0].Suit
	for _, t := range hand {
		counts[t]++
		if t.Suit != first {
			pure = false
		}
	}
	for _, m := range melds {
		for _, t := range m.Tiles {
			counts[t]++
			if t.Suit != first {
				pure = false
			}
		}
	}
	roots := 0
	for _, n := range counts {
		if n == 4 {
			roots++
		}
	}
	fan := 0
	if pure {
		fan += 4
	}
	if len(melds) == 0 && sevenPairs(hand) {
		fan += 4 + roots*dragonBonus
	} else {
		triplets := allTripletShape(hand, 4-len(melds))
		for _, m := range melds {
			if m.Kind == "chi" {
				triplets = false
			}
		}
		if triplets {
			fan += 2
		}
		if len(melds) == 4 && len(hand) == 2 {
			fan += 2
		}
	}
	if fan == 0 {
		fan = 1
	}
	return fan + roots
}

func (g *Game) capFan(fan int) int {
	if g.optionsConfig.FanCap > 0 && fan > g.optionsConfig.FanCap {
		return g.optionsConfig.FanCap
	}
	return fan
}

func (g *Game) winFan(seat int, hand []Tile, selfDraw bool) int {
	fan := patternFan(hand, g.melds[seat], g.optionsConfig.DragonPairBonus)
	if g.phase == "rob_gang" {
		fan++
	}
	if selfDraw && g.gangDraw {
		fan++
	}
	if !selfDraw && g.phase != "rob_gang" && g.gangDiscard {
		fan++
	}
	if len(g.wall) == 0 {
		fan++
	}
	if selfDraw && g.optionsConfig.SelfDrawBonus {
		fan++
	}
	// Heaven: dealer wins before any discard. Earth: the first discard wins.
	opening := true
	for _, melds := range g.melds {
		if len(melds) != 0 {
			opening = false
			break
		}
	}
	if g.optionsConfig.HeavenlyEarthlyCap && opening && ((selfDraw && seat == g.dealer && g.discardCount == 0) || (!selfDraw && g.discardCount == 1 && g.phase != "rob_gang")) {
		return g.optionsConfig.FanCap
	}
	return g.capFan(fan)
}

// Only the player's own tiles and public tiles determine dead waits. Hidden
// opponents' hands must not influence the bot's knowledge or ready status.
func (g *Game) visibleCount(seat int, tile Tile) int {
	n := countTile(g.hands[seat], tile)
	for i := 0; i < Players; i++ {
		n += countTile(g.discards[i], tile)
		for _, meld := range g.melds[i] {
			n += countTile(meld.Tiles, tile)
		}
	}
	return n
}
