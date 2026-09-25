package sunjiajia

import (
	"fmt"
	"sort"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/cards"
	"github.com/17hwliao/table-card-independent/internal/landlord"
	"github.com/17hwliao/table-card-independent/internal/liarbar"
)

// Agent is a self-contained heuristic player. Its choices are reproducible for
// the same hand and table position, which makes decisions explainable.
type Agent struct{}

type Table struct {
	Seat      int
	Landlord  int
	CardsLeft [3]int
	LastTrick *landlord.Trick
}

type Candidate struct {
	Cards   []cards.Card
	Pattern landlord.Hand
	Score   float64
}

// ChooseLiarBarPlay prefers truthful cards and otherwise makes a small bluff
// to preserve stronger cards. It returns IDs from the private hand view.
func (Agent) ChooseLiarBarPlay(hand []liarbar.Card, target liarbar.Rank) []uint8 {
	truthful := make([]liarbar.Card, 0, len(hand))
	for _, card := range hand {
		if card.Rank == target || card.Rank == liarbar.Joker {
			truthful = append(truthful, card)
		}
	}
	pool := hand
	if len(truthful) > 0 {
		pool = truthful
	}
	count := min(len(pool), 2)
	if count == 0 {
		return nil
	}
	ids := make([]uint8, count)
	for i := 0; i < count; i++ {
		ids[i] = pool[i].ID
	}
	return ids
}

// ShouldChallenge is deliberately risk-aware: larger plays are more likely to
// contain a lie, while a bot near the sixth shot demands stronger evidence.
func (Agent) ShouldChallenge(view liarbar.Snapshot, playerID string) bool {
	if view.Pending == nil || !view.Pending.CanCall || view.Phase != liarbar.RoundActive {
		return false
	}
	selfShots := 0
	for _, player := range view.Players {
		if player.ID == playerID {
			selfShots = player.Shots
			break
		}
	}
	confidence := 0.30 + float64(view.Pending.Count-1)*0.15
	if view.Pending.Count == 3 {
		confidence += 0.08
	}
	threshold := 0.53
	if selfShots >= 4 {
		threshold += 0.18
	}
	if seat := view.Pending.Seat; seat >= 0 && seat < len(view.Players) && view.Players[seat].Shots >= 4 {
		confidence += 0.10
	}
	return confidence >= threshold
}

func (Agent) Bid(hand []cards.Card) int {
	strength := 0.0
	counts := make(map[cards.Rank]int)
	for _, card := range hand {
		counts[card.Rank]++
		switch card.Rank {
		case cards.BigJoker:
			strength += 2.2
		case cards.SmallJoker:
			strength += 1.7
		case cards.Two:
			strength += 0.75
		case cards.Ace:
			strength += 0.42
		case cards.King:
			strength += 0.2
		}
	}
	for _, count := range counts {
		switch count {
		case 2:
			strength += 0.16
		case 3:
			strength += 0.55
		case 4:
			strength += 1.5
		}
	}
	switch {
	case strength >= 9.0:
		return 3
	case strength >= 7.2:
		return 2
	case strength >= 5.4:
		return 1
	default:
		return 0
	}
}

// Choose returns a card selection to lead, beat the table, or pass. Farmer
// teammates avoid taking a trick from one another unless an opponent is close
// to winning the round.
func (a Agent) Choose(hand []cards.Card, table Table) []cards.Card {
	trick := table.LastTrick
	if trick != nil && onSameTeam(table.Seat, trick.Seat, table.Landlord) && table.Seat != trick.Seat {
		if table.CardsLeft[trick.Seat] > 2 {
			return nil
		}
	}

	all := candidates(hand)
	legal := make([]Candidate, 0, len(all))
	for _, candidate := range all {
		if trick != nil && !landlord.Beats(candidate.Pattern, trick.Pattern) {
			continue
		}
		remaining, ok := remove(hand, candidate.Cards)
		if !ok {
			continue
		}
		candidate.Score = handCost(remaining) - 0.2*float64(len(candidate.Cards))
		if trick != nil {
			candidate.Score += float64(candidate.Pattern.MainRank-cards.Three) * 0.11
			if candidate.Pattern.Kind == landlord.Bomb {
				candidate.Score += 3.8
			} else if candidate.Pattern.Kind == landlord.Rocket {
				candidate.Score += 5.5
			}
			if onSameTeam(table.Seat, trick.Seat, table.Landlord) {
				candidate.Score -= 0.3
			}
		}
		candidate.Cards = append([]cards.Card(nil), candidate.Cards...)
		legal = append(legal, candidate)
	}
	if len(legal) == 0 {
		return nil
	}
	sort.SliceStable(legal, func(i, j int) bool { return legal[i].Score < legal[j].Score })
	chosen := legal[0]
	if trick != nil && chosen.Pattern.Kind == landlord.Bomb {
		for _, candidate := range legal {
			if candidate.Pattern.Kind != landlord.Bomb && candidate.Pattern.Kind != landlord.Rocket {
				chosen = candidate
				break
			}
		}
	}
	return chosen.Cards
}

func candidates(hand []cards.Card) []Candidate {
	available := make(map[cards.Rank]int)
	byRank := make(map[cards.Rank][]cards.Card)
	for _, card := range hand {
		available[card.Rank]++
		byRank[card.Rank] = append(byRank[card.Rank], card)
	}
	templates := make(map[string]map[cards.Rank]int)
	add := func(set map[cards.Rank]int) {
		key := templateKey(set)
		templates[key] = set
	}
	for rank := cards.Three; rank <= cards.BigJoker; rank++ {
		if available[rank] > 0 {
			add(map[cards.Rank]int{rank: 1})
		}
		if rank < cards.SmallJoker {
			for copies := 2; copies <= min(available[rank], 4); copies++ {
				add(map[cards.Rank]int{rank: copies})
			}
		}
	}
	if available[cards.SmallJoker] > 0 && available[cards.BigJoker] > 0 {
		add(map[cards.Rank]int{cards.SmallJoker: 1, cards.BigJoker: 1})
	}
	for rank := cards.Three; rank <= cards.Ace; rank++ {
		if available[rank] < 3 {
			continue
		}
		for _, kicker := range allRanks() {
			if kicker != rank && available[kicker] > 0 {
				add(map[cards.Rank]int{rank: 3, kicker: 1})
			}
			if kicker != rank && kicker < cards.SmallJoker && available[kicker] >= 2 {
				add(map[cards.Rank]int{rank: 3, kicker: 2})
			}
		}
	}
	for rank := cards.Three; rank <= cards.Ace; rank++ {
		if available[rank] < 4 {
			continue
		}
		for i, first := range allRanks() {
			for _, second := range allRanks()[i:] {
				set := map[cards.Rank]int{rank: 4}
				set[first]++
				set[second]++
				if first == rank || second == rank || exceeds(set, available) {
					continue
				}
				add(set)
			}
		}
		for i, first := range lowRanks() {
			for _, second := range lowRanks()[i+1:] {
				set := map[cards.Rank]int{rank: 4, first: 2, second: 2}
				if first == rank || second == rank || exceeds(set, available) {
					continue
				}
				add(set)
			}
		}
	}
	for length := 5; length <= 12; length++ {
		addRuns(add, available, length, 1)
	}
	for length := 3; length <= 10; length++ {
		addRuns(add, available, length, 2)
	}
	for length := 2; length <= 6; length++ {
		addRuns(add, available, length, 3)
	}
	addAirplanes(add, available, 1)
	addAirplanes(add, available, 2)

	result := make([]Candidate, 0, len(templates))
	for _, template := range templates {
		play := make([]cards.Card, 0, templateSize(template))
		for _, rank := range sortedTemplateRanks(template) {
			play = append(play, byRank[rank][:template[rank]]...)
		}
		pattern, err := landlord.Classify(play)
		if err != nil {
			continue
		}
		result = append(result, Candidate{Cards: play, Pattern: pattern})
	}
	return result
}

func addAirplanes(add func(map[cards.Rank]int), available map[cards.Rank]int, wingCopies int) {
	for length := 2; length <= 6; length++ {
		for low := cards.Three; low+cards.Rank(length)-1 <= cards.Ace; low++ {
			core := make(map[cards.Rank]int, length)
			valid := true
			for offset := 0; offset < length; offset++ {
				rank := low + cards.Rank(offset)
				if available[rank] < 3 {
					valid = false
					break
				}
				core[rank] = 3
			}
			if !valid {
				continue
			}
			if wingCopies == 1 {
				addSingleWings(add, available, core, length)
			} else {
				addPairWings(add, available, core, length)
			}
		}
	}
}

func addSingleWings(add func(map[cards.Rank]int), available, core map[cards.Rank]int, count int) {
	var choose func(start cards.Rank, left int, wings map[cards.Rank]int)
	choose = func(start cards.Rank, left int, wings map[cards.Rank]int) {
		if left == 0 {
			set := cloneCounts(core)
			for rank, amount := range wings {
				set[rank] += amount
			}
			add(set)
			return
		}
		for rank := start; rank <= cards.BigJoker; rank++ {
			if core[rank] != 0 || wings[rank] >= available[rank] {
				continue
			}
			wings[rank]++
			choose(rank, left-1, wings)
			wings[rank]--
		}
	}
	choose(cards.Three, count, make(map[cards.Rank]int))
}

func addPairWings(add func(map[cards.Rank]int), available, core map[cards.Rank]int, pairCount int) {
	ranks := lowRanks()
	var choose func(start, left int, wings map[cards.Rank]int)
	choose = func(start, left int, wings map[cards.Rank]int) {
		if left == 0 {
			set := cloneCounts(core)
			for rank, amount := range wings {
				set[rank] += amount
			}
			add(set)
			return
		}
		for i := start; i < len(ranks); i++ {
			rank := ranks[i]
			if core[rank] != 0 || available[rank] < 2 {
				continue
			}
			wings[rank] = 2
			choose(i+1, left-1, wings)
			delete(wings, rank)
		}
	}
	choose(0, pairCount, make(map[cards.Rank]int))
}

func cloneCounts(source map[cards.Rank]int) map[cards.Rank]int {
	copy := make(map[cards.Rank]int, len(source))
	for rank, count := range source {
		copy[rank] = count
	}
	return copy
}

func addRuns(add func(map[cards.Rank]int), available map[cards.Rank]int, length, copies int) {
	for low := cards.Three; low+cards.Rank(length)-1 <= cards.Ace; low++ {
		set := make(map[cards.Rank]int, length)
		valid := true
		for offset := 0; offset < length; offset++ {
			rank := low + cards.Rank(offset)
			if available[rank] < copies {
				valid = false
				break
			}
			set[rank] = copies
		}
		if valid {
			add(set)
		}
	}
}

func allRanks() []cards.Rank {
	ranks := make([]cards.Rank, 0, 15)
	for rank := cards.Three; rank <= cards.BigJoker; rank++ {
		ranks = append(ranks, rank)
	}
	return ranks
}

func lowRanks() []cards.Rank {
	ranks := make([]cards.Rank, 0, 13)
	for rank := cards.Three; rank <= cards.Ace; rank++ {
		ranks = append(ranks, rank)
	}
	return ranks
}

func exceeds(set map[cards.Rank]int, available map[cards.Rank]int) bool {
	for rank, count := range set {
		if count > available[rank] {
			return true
		}
	}
	return false
}

func templateKey(set map[cards.Rank]int) string {
	ranks := sortedTemplateRanks(set)
	var key strings.Builder
	for _, rank := range ranks {
		fmt.Fprintf(&key, "%02d:%d;", rank, set[rank])
	}
	return key.String()
}

func sortedTemplateRanks(set map[cards.Rank]int) []cards.Rank {
	ranks := make([]cards.Rank, 0, len(set))
	for rank := range set {
		ranks = append(ranks, rank)
	}
	sort.Slice(ranks, func(i, j int) bool { return ranks[i] < ranks[j] })
	return ranks
}

func templateSize(set map[cards.Rank]int) int {
	size := 0
	for _, count := range set {
		size += count
	}
	return size
}

func remove(hand, selected []cards.Card) ([]cards.Card, bool) {
	used := make([]bool, len(hand))
	for _, wanted := range selected {
		found := false
		for i, held := range hand {
			if !used[i] && held == wanted {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	remaining := make([]cards.Card, 0, len(hand)-len(selected))
	for i, card := range hand {
		if !used[i] {
			remaining = append(remaining, card)
		}
	}
	return remaining, true
}

func handCost(hand []cards.Card) float64 {
	counts := make(map[cards.Rank]int)
	for _, card := range hand {
		counts[card.Rank]++
	}
	cost := 0.0
	for rank, count := range counts {
		switch count {
		case 1:
			cost += 1.9 - float64(rank-cards.Three)*0.075
		case 2:
			cost += 0.72 - float64(rank-cards.Three)*0.018
		case 3:
			cost += 0.2
		case 4:
			cost -= 2.7
		}
	}
	if counts[cards.SmallJoker] > 0 && counts[cards.BigJoker] > 0 {
		cost -= 3.4
	}
	return cost
}

func onSameTeam(a, b, landlordSeat int) bool {
	return a != landlordSeat && b != landlordSeat
}
