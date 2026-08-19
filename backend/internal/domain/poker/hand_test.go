package poker

import (
	"testing"
)

func c(s Suit, r Rank) Card { return Card{Suit: s, Rank: r} }

func assertCat(t *testing.T, got Hand, want Category) {
	t.Helper()
	if got.Cat != want {
		t.Fatalf("expected %v, got %v (%v)", want, got.Cat, got.Tiers)
	}
}

func TestStraightFlush(t *testing.T) {
	h := evaluate5([]Card{c(Spades, 13), c(Spades, 12), c(Spades, 11), c(Spades, 10), c(Spades, 9)})
	assertCat(t, h, StraightFlush)
	if h.Tiers[0] != 13 {
		t.Fatalf("high card got %v", h.Tiers[0])
	}
}

func TestRoyalFlushIsStraightFlush(t *testing.T) {
	h := evaluate5([]Card{c(Hearts, 14), c(Hearts, 13), c(Hearts, 12), c(Hearts, 11), c(Hearts, 10)})
	assertCat(t, h, StraightFlush)
}

func TestWheelStraightFlush(t *testing.T) {
	h := evaluate5([]Card{c(Diamonds, 14), c(Diamonds, 2), c(Diamonds, 3), c(Diamonds, 4), c(Diamonds, 5)})
	assertCat(t, h, StraightFlush)
	if h.Tiers[0] != 5 {
		t.Fatalf("wheel high card got %v", h.Tiers[0])
	}
}

func TestFourOfAKind(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 7), c(Diamonds, 7), c(Hearts, 7), c(Spades, 7), c(Clubs, 10)})
	assertCat(t, h, FourOfAKind)
	if h.Tiers[0] != 7 || h.Tiers[1] != 10 {
		t.Fatalf("tiers got %v", h.Tiers)
	}
}

func TestFullHouse(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 9), c(Diamonds, 9), c(Hearts, 9), c(Spades, 4), c(Clubs, 4)})
	assertCat(t, h, FullHouse)
	if h.Tiers[0] != 9 || h.Tiers[1] != 4 {
		t.Fatalf("tiers got %v", h.Tiers)
	}
}

func TestFlush(t *testing.T) {
	h := evaluate5([]Card{c(Hearts, 14), c(Hearts, 10), c(Hearts, 7), c(Hearts, 5), c(Hearts, 2)})
	assertCat(t, h, Flush)
}

func TestFlushBeatsStraight(t *testing.T) {
	flush := evaluate5([]Card{c(Hearts, 14), c(Hearts, 10), c(Hearts, 7), c(Hearts, 5), c(Hearts, 2)})
	straight := evaluate5([]Card{c(Clubs, 9), c(Diamonds, 8), c(Hearts, 7), c(Spades, 6), c(Clubs, 5)})
	if !better(flush, straight) {
		t.Fatalf("flush should beat straight")
	}
}

func TestStraight(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 9), c(Diamonds, 8), c(Hearts, 7), c(Spades, 6), c(Clubs, 5)})
	assertCat(t, h, Straight)
	if h.Tiers[0] != 9 {
		t.Fatalf("high got %v", h.Tiers[0])
	}
}

func TestWheelStraight(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 14), c(Diamonds, 2), c(Hearts, 3), c(Spades, 4), c(Clubs, 5)})
	assertCat(t, h, Straight)
	if h.Tiers[0] != 5 {
		t.Fatalf("wheel high got %v", h.Tiers[0])
	}
}

func TestThreeOfAKind(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 5), c(Diamonds, 5), c(Hearts, 5), c(Spades, 9), c(Clubs, 3)})
	assertCat(t, h, ThreeOfAKind)
}

func TestTwoPair(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 8), c(Diamonds, 8), c(Hearts, 3), c(Spades, 3), c(Clubs, 9)})
	assertCat(t, h, TwoPair)
	if h.Tiers[0] != 8 || h.Tiers[1] != 3 || h.Tiers[2] != 9 {
		t.Fatalf("tiers got %v", h.Tiers)
	}
}

func TestOnePair(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 2), c(Diamonds, 2), c(Hearts, 13), c(Spades, 7), c(Clubs, 4)})
	assertCat(t, h, OnePair)
}

func TestHighCard(t *testing.T) {
	h := evaluate5([]Card{c(Clubs, 2), c(Diamonds, 4), c(Hearts, 6), c(Spades, 8), c(Clubs, 10)})
	assertCat(t, h, HighCard)
}

func TestPairKickerTieBreak(t *testing.T) {
	a := evaluate5([]Card{c(Clubs, 2), c(Diamonds, 2), c(Hearts, 13), c(Spades, 7), c(Clubs, 4)})
	b := evaluate5([]Card{c(Spades, 2), c(Hearts, 2), c(Diamonds, 12), c(Clubs, 7), c(Spades, 4)})
	if !better(a, b) {
		t.Fatalf("pair with K kicker should beat pair with Q kicker")
	}
	if better(b, a) {
		t.Fatalf("q kicker should not beat k kicker")
	}
}

func TestEvaluateAllBestOfSeven(t *testing.T) {
	hole := [2]Card{c(Spades, 9), c(Spades, 8)}
	board := []Card{c(Spades, 13), c(Spades, 7), c(Spades, 6), c(Diamonds, 4), c(Clubs, 2)}
	h := EvaluateAll(hole, board)
	assertCat(t, h, Flush)
}

func TestEvaluateAllFindsBoardTripsNotPair(t *testing.T) {
	hole := [2]Card{c(Clubs, 2), c(Diamonds, 5)}
	board := []Card{c(Spades, 8), c(Hearts, 8), c(Clubs, 8), c(Diamonds, 3), c(Clubs, 3)}
	h := EvaluateAll(hole, board)
	assertCat(t, h, FullHouse)
}

func TestDeckHasFiftyTwoUnique(t *testing.T) {
	d := NewDeck()
	seen := map[Card]bool{}
	for _, card := range d.cards {
		if seen[card] {
			t.Fatalf("duplicate card %v", card)
		}
		seen[card] = true
	}
	if len(seen) != 52 {
		t.Fatalf("expected 52 unique, got %d", len(seen))
	}
}

func TestDeckShuffleChangesOrder(t *testing.T) {
	d := NewDeck()
	before := make([]Card, len(d.cards))
	copy(before, d.cards)
	d.Shuffle(func(n int) int { return 0 })
	changed := false
	for i := range before {
		if before[i] != d.cards[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatalf("shuffle did not change ordering")
	}
}

func TestDeckDraw(t *testing.T) {
	d := NewDeck()
	first, err := d.Draw()
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if len(d.cards) != 51 {
		t.Fatalf("expected 51 cards after draw, got %d", len(d.cards))
	}
	_ = first
}

func TestEvaluateBestPreflopPocketPair(t *testing.T) {
	h, combo := EvaluateBest([2]Card{c(Spades, 13), c(Hearts, 13)}, nil)
	if h.Cat != OnePair {
		t.Fatalf("expected a pair preflop, got %v", h.Cat)
	}
	if h.Tiers[0] != 13 {
		t.Fatalf("expected kings, got %v", h.Tiers[0])
	}
	if len(combo) != 2 {
		t.Fatalf("expected both hole cards in the combination, got %d", len(combo))
	}
}

func TestEvaluateBestPreflopHighCard(t *testing.T) {
	h, combo := EvaluateBest([2]Card{c(Clubs, 9), c(Diamonds, 14)}, nil)
	if h.Cat != HighCard {
		t.Fatalf("expected high card preflop, got %v", h.Cat)
	}
	if h.Tiers[0] != 14 {
		t.Fatalf("expected the ace to lead, got %v", h.Tiers[0])
	}
	if len(combo) != 1 || combo[0].Rank != 14 {
		t.Fatalf("expected the ace as the combination, got %v", combo)
	}
}

func TestEvaluateBestWithoutHoleCards(t *testing.T) {
	if _, combo := EvaluateBest([2]Card{}, nil); combo != nil {
		t.Fatalf("expected no combination without hole cards, got %v", combo)
	}
}
