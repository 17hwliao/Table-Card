package landlord

import (
	"errors"
	"fmt"
	"sort"

	"github.com/17hwliao/table-card-independent/internal/cards"
)

type Kind uint8

const (
	Invalid Kind = iota
	Pass
	Single
	Pair
	Triple
	TripleSingle
	TriplePair
	Straight
	ConsecutivePairs
	Airplane
	AirplaneSingles
	AirplanePairs
	FourWithSingles
	FourWithPairs
	Bomb
	Rocket
)

type Hand struct {
	Kind      Kind
	MainRank  cards.Rank
	RunLength int
	CardCount int
}

var ErrInvalidHand = errors.New("不构成合法牌型")

func Classify(play []cards.Card) (Hand, error) {
	if len(play) == 0 {
		return Hand{Kind: Pass}, nil
	}
	counts := make(map[cards.Rank]int, len(play))
	for _, c := range play {
		counts[c.Rank]++
	}
	ranks := sortedRanks(counts)
	n := len(play)
	result := Hand{CardCount: n}

	switch {
	case n == 1:
		return Hand{Kind: Single, MainRank: play[0].Rank, CardCount: n}, nil
	case n == 2 && counts[cards.SmallJoker] == 1 && counts[cards.BigJoker] == 1:
		return Hand{Kind: Rocket, MainRank: cards.BigJoker, CardCount: n}, nil
	case n == 2 && len(ranks) == 1:
		return withMain(result, Pair, ranks[0], 1), nil
	case n == 3 && len(ranks) == 1:
		return withMain(result, Triple, ranks[0], 1), nil
	case n == 4 && len(ranks) == 1:
		return withMain(result, Bomb, ranks[0], 1), nil
	case n == 4 && hasGroup(counts, 3):
		return withMain(result, TripleSingle, rankWithCount(counts, 3), 1), nil
	case n == 5 && hasGroup(counts, 3) && hasGroup(counts, 2):
		return withMain(result, TriplePair, rankWithCount(counts, 3), 1), nil
	case isRun(counts, ranks, 1, 5):
		return withMain(result, Straight, ranks[len(ranks)-1], len(ranks)), nil
	case isRun(counts, ranks, 2, 3):
		return withMain(result, ConsecutivePairs, ranks[len(ranks)-1], len(ranks)), nil
	}

	// Search every valid core length. A longer run of triples can also supply
	// single wings; selecting only the maximal run rejects valid airplanes.
	for _, shape := range []struct {
		copies int
		kind   Kind
	}{{3, Airplane}, {4, AirplaneSingles}, {5, AirplanePairs}} {
		if n%shape.copies != 0 || n/shape.copies < 2 {
			continue
		}
		length := n / shape.copies
		for low := cards.Three; int(low)+length-1 <= int(cards.Ace); low++ {
			core := tripleCore{low: low, high: low + cards.Rank(length-1), length: length}
			valid := true
			for rank := core.low; rank <= core.high; rank++ {
				if counts[rank] < 3 {
					valid = false
					break
				}
			}
			if valid && (shape.kind != AirplanePairs || remainingArePairs(counts, core)) {
				return withMain(result, shape.kind, core.high, length), nil
			}
		}
	}

	if rank, ok := rankWithCountOK(counts, 4); ok {
		left := n - 4
		if left == 2 {
			return withMain(result, FourWithSingles, rank, 1), nil
		}
		if left == 4 && remainingAreTwoPairs(counts, rank) {
			return withMain(result, FourWithPairs, rank, 1), nil
		}
	}
	return Hand{}, fmt.Errorf("%w: %d 张牌", ErrInvalidHand, n)
}

func Beats(candidate, target Hand) bool {
	if candidate.Kind == Pass || candidate.Kind == Invalid || target.Kind == Pass || target.Kind == Invalid {
		return false
	}
	if candidate.Kind == Rocket {
		return target.Kind != Rocket
	}
	if target.Kind == Rocket {
		return false
	}
	if candidate.Kind == Bomb && target.Kind != Bomb {
		return true
	}
	if candidate.Kind != target.Kind || candidate.RunLength != target.RunLength {
		return false
	}
	return candidate.MainRank > target.MainRank
}

func withMain(h Hand, kind Kind, rank cards.Rank, run int) Hand {
	h.Kind, h.MainRank, h.RunLength = kind, rank, run
	return h
}

func sortedRanks(counts map[cards.Rank]int) []cards.Rank {
	ranks := make([]cards.Rank, 0, len(counts))
	for rank := range counts {
		ranks = append(ranks, rank)
	}
	sort.Slice(ranks, func(i, j int) bool { return ranks[i] < ranks[j] })
	return ranks
}

func hasGroup(counts map[cards.Rank]int, size int) bool {
	_, ok := rankWithCountOK(counts, size)
	return ok
}

func rankWithCount(counts map[cards.Rank]int, size int) cards.Rank {
	rank, _ := rankWithCountOK(counts, size)
	return rank
}

func rankWithCountOK(counts map[cards.Rank]int, size int) (cards.Rank, bool) {
	for rank, count := range counts {
		if count == size {
			return rank, true
		}
	}
	return 0, false
}

func isRun(counts map[cards.Rank]int, ranks []cards.Rank, copies, minimum int) bool {
	if len(ranks) < minimum || ranks[len(ranks)-1] > cards.Ace {
		return false
	}
	for i, rank := range ranks {
		if counts[rank] != copies || (i > 0 && rank != ranks[i-1]+1) {
			return false
		}
	}
	return true
}

type tripleCore struct {
	low, high cards.Rank
	length    int
}

func remainingArePairs(counts map[cards.Rank]int, core tripleCore) bool {
	for rank, count := range counts {
		if rank >= core.low && rank <= core.high {
			count -= 3
		}
		if count < 0 || count%2 != 0 || (count > 0 && rank >= cards.SmallJoker) {
			return false
		}
	}
	return true
}

func remainingAreTwoPairs(counts map[cards.Rank]int, quad cards.Rank) bool {
	pairs := 0
	for rank, count := range counts {
		if rank == quad {
			count -= 4
		}
		if count < 0 || count%2 != 0 || count > 2 || (count > 0 && rank >= cards.SmallJoker) {
			return false
		}
		pairs += count / 2
	}
	return pairs == 2
}
