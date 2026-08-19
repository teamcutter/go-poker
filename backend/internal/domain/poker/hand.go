package poker

import "sort"

type Category int

const (
	HighCard Category = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
)

func (c Category) String() string {
	switch c {
	case HighCard:
		return "High Card"
	case OnePair:
		return "Pair"
	case TwoPair:
		return "Two Pair"
	case ThreeOfAKind:
		return "Trips"
	case Straight:
		return "Straight"
	case Flush:
		return "Flush"
	case FullHouse:
		return "Full House"
	case FourOfAKind:
		return "Quads"
	case StraightFlush:
		return "Straight Flush"
	}
	return "?"
}

type Hand struct {
	Cat   Category
	Tiers [5]Rank
}

func evaluate5(cards []Card) Hand {
	if len(cards) != 5 {
		return Hand{Cat: HighCard}
	}
	ranks := make([]int, len(cards))
	isFlush := true
	for i, c := range cards {
		ranks[i] = int(c.Rank)
		if i > 0 && cards[i].Suit != cards[i-1].Suit {
			isFlush = false
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ranks)))

	unique := uniqueRanks(ranks)
	straightHigh, isStraight := straightHigh(unique)

	groups := groupRanks(ranks)

	switch {
	case isFlush && isStraight:
		return Hand{Cat: StraightFlush, Tiers: [5]Rank{Rank(straightHigh)}}
	case groups[0].count == 4:
		h := Hand{Cat: FourOfAKind}
		h.Tiers[0] = groups[0].rank
		h.Tiers[1] = groups[1].rank
		return h
	case groups[0].count == 3 && len(groups) >= 2 && groups[1].count == 2:
		return Hand{Cat: FullHouse, Tiers: [5]Rank{groups[0].rank, groups[1].rank}}
	case isFlush:
		h := Hand{Cat: Flush}
		for i := 0; i < 5; i++ {
			h.Tiers[i] = groups[i].rank
		}
		return h
	case isStraight:
		return Hand{Cat: Straight, Tiers: [5]Rank{Rank(straightHigh)}}
	case groups[0].count == 3:
		h := Hand{Cat: ThreeOfAKind}
		h.Tiers[0] = groups[0].rank
		idx := 1
		for i := 1; i < len(groups) && idx < 5; i++ {
			h.Tiers[idx] = groups[i].rank
			idx++
		}
		return h
	case groups[0].count == 2 && len(groups) >= 2 && groups[1].count == 2:
		h := Hand{Cat: TwoPair}
		h.Tiers[0] = groups[0].rank
		h.Tiers[1] = groups[1].rank
		h.Tiers[2] = groups[2].rank
		return h
	case groups[0].count == 2:
		h := Hand{Cat: OnePair}
		h.Tiers[0] = groups[0].rank
		idx := 1
		for i := 1; i < len(groups) && idx < 5; i++ {
			h.Tiers[idx] = groups[i].rank
			idx++
		}
		return h
	default:
		h := Hand{Cat: HighCard}
		for i := 0; i < 5; i++ {
			h.Tiers[i] = groups[i].rank
		}
		return h
	}
}

func EvaluateAll(hole [2]Card, board []Card) Hand {
	best, _ := EvaluateBest(hole, board)
	return best
}

// evaluateHole ranks the two hole cards on their own, so a player can see what
// they are holding before any community card is dealt.
func evaluateHole(hole [2]Card) (Hand, []Card) {
	high, low := hole[0], hole[1]
	if low.Rank > high.Rank {
		high, low = low, high
	}
	if high.Rank == low.Rank {
		return Hand{Cat: OnePair, Tiers: [5]Rank{high.Rank}}, []Card{high, low}
	}
	return Hand{Cat: HighCard, Tiers: [5]Rank{high.Rank, low.Rank}}, []Card{high}
}

func EvaluateBest(hole [2]Card, board []Card) (Hand, []Card) {
	if hole[0] == (Card{}) || hole[1] == (Card{}) {
		return Hand{Cat: HighCard}, nil
	}
	if len(board) < 3 {
		return evaluateHole(hole)
	}
	seven := make([]Card, 0, 7)
	seven = append(seven, hole[0], hole[1])
	seven = append(seven, board...)

	indices := make([]int, 0, 5)
	best := Hand{Cat: HighCard}
	var bestCombo []Card
	bestSet := false
	var pick func(start int)
	pick = func(start int) {
		if len(indices) == 5 {
			combo := make([]Card, 5)
			for i, idx := range indices {
				combo[i] = seven[idx]
			}
			h := evaluate5(combo)
			if !bestSet || better(h, best) {
				best = h
				bestCombo = combo
				bestSet = true
			}
			return
		}
		for i := start; i < len(seven); i++ {
			indices = append(indices, i)
			pick(i + 1)
			indices = indices[:len(indices)-1]
		}
	}
	pick(0)
	return best, bestCombo
}

func better(a, b Hand) bool {
	if a.Cat != b.Cat {
		return a.Cat > b.Cat
	}
	for i := 0; i < 5; i++ {
		if a.Tiers[i] != b.Tiers[i] {
			return a.Tiers[i] > b.Tiers[i]
		}
	}
	return false
}

type rankGroup struct {
	rank  Rank
	count int
}

func groupRanks(ranks []int) []rankGroup {
	counts := map[Rank]int{}
	for _, r := range ranks {
		counts[Rank(r)]++
	}
	out := make([]rankGroup, 0, len(counts))
	for r, c := range counts {
		out = append(out, rankGroup{rank: r, count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		return out[i].rank > out[j].rank
	})
	return out
}

func uniqueRanks(ranks []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, r := range ranks {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

func straightHigh(unique []int) (int, bool) {
	if len(unique) < 5 {
		return 0, false
	}
	if unique[0] == 14 && unique[1] == 5 && unique[2] == 4 && unique[3] == 3 && unique[4] == 2 {
		return 5, true
	}
	run := 1
	high := unique[0]
	for i := 1; i < len(unique); i++ {
		if unique[i] == unique[i-1]-1 {
			run++
			if run >= 5 {
				high = unique[i-4]
			}
		} else if unique[i] != unique[i-1] {
			run = 1
		}
	}
	if run >= 5 {
		return high, true
	}
	return 0, false
}
