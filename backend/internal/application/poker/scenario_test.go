package poker

import (
	"testing"
	"time"

	domainpoker "github.com/teamcutter/go-poker/internal/domain/poker"
)

func TestScenario20010TurnTimeoutBoundary(t *testing.T) {
	s := NewService(nil)
	base := time.Now()
	s.now = func() time.Time { return base }
	tb, err := s.CreateTable(2, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"u1", "u2"} {
		if err := s.Join(tb.Code, id, 200); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.StartHand(tb.Code, tb.Players[tb.NextStarterSeat()].ID); err != nil {
		t.Fatal(err)
	}
	acting := tb.Acting
	if touched := s.EnforceTurns(base.Add(TurnLimit - time.Second)); len(touched) != 0 {
		t.Fatalf("timeout before 30 seconds: %v", touched)
	}
	if tb.Acting != acting || tb.Players[acting].Folded {
		t.Fatal("the acting player lost the turn before the deadline")
	}
	if touched := s.EnforceTurns(base.Add(TurnLimit)); len(touched) != 1 || touched[0] != tb.Code {
		t.Fatalf("timeout at 30 seconds: %v", touched)
	}
	if !tb.Players[acting].Folded || tb.Phase != domainpoker.PhaseComplete {
		t.Fatalf("expected auto-fold and completed hand, phase=%v folded=%v", tb.Phase, tb.Players[acting].Folded)
	}
}

func TestScenario20012AwayTimeoutBoundary(t *testing.T) {
	s := NewService(nil)
	base := time.Now()
	s.now = func() time.Time { return base }
	tb, err := s.CreateTable(2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Join(tb.Code, "u1", 400); err != nil {
		t.Fatal(err)
	}
	s.MarkPresent(tb.Code, "u1")
	s.MarkAway(tb.Code, "u1")
	s.ReapIdle(base.Add(AwayGrace - time.Second))
	if len(tb.Players) != 1 || s.Bankroll("u1") != defaultChips-400 {
		t.Fatal("seat or bankroll changed before the away deadline")
	}
	s.ReapIdle(base.Add(AwayGrace))
	if len(tb.Players) != 0 || s.Bankroll("u1") != defaultChips {
		t.Fatalf("away seat was not released and refunded: seats=%d bankroll=%d", len(tb.Players), s.Bankroll("u1"))
	}
}
