package poker

import (
	"testing"
	"time"

	domainpoker "github.com/teamcutter/go-poker/internal/domain/poker"
)

func TestJoinDeductsBuyInFromBankroll(t *testing.T) {
	s := NewService(nil)
	tb, err := s.CreateTable(6, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Join(tb.Code, "u1", 200); err != nil {
		t.Fatal(err)
	}
	if got := s.Bankroll("u1"); got != defaultChips-200 {
		t.Fatalf("expected bankroll %d, got %d", defaultChips-200, got)
	}
	if tb.Players[0].Stack != 200 {
		t.Fatalf("expected a 200 stack at the table, got %d", tb.Players[0].Stack)
	}
}

func TestJoinRejectedWithoutChips(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", defaultChips); err != nil {
		t.Fatal(err)
	}
	tb.Players[0].Stack = 0
	s.Leave(tb.Code, "u1")

	other, _ := s.CreateTable(6, 10)
	if err := s.Join(other.Code, "u1", 100); err != ErrNotEnoughChips {
		t.Fatalf("expected ErrNotEnoughChips, got %v", err)
	}
}

func TestReapIdleClosesEmptyTableAfterGrace(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	start := time.Now()
	if closed := s.ReapIdle(start); len(closed) != 0 {
		t.Fatalf("table closed before the grace period: %v", closed)
	}
	closed := s.ReapIdle(start.Add(IdleGrace))
	if len(closed) != 1 || closed[0] != tb.Code {
		t.Fatalf("expected %s to be closed, got %v", tb.Code, closed)
	}
	if _, err := s.Get(tb.Code); err != ErrTableNotFound {
		t.Fatalf("expected the table to be gone, got %v", err)
	}
}

func TestReapIdleKeepsOccupiedTable(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 200); err != nil {
		t.Fatal(err)
	}
	s.MarkPresent(tb.Code, "u1")
	if closed := s.ReapIdle(time.Now().Add(time.Hour)); len(closed) != 0 {
		t.Fatalf("an occupied table was closed: %v", closed)
	}
}

func TestLeaveReturnsStackAndStartsGrace(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 400); err != nil {
		t.Fatal(err)
	}
	s.Leave(tb.Code, "u1")
	if got := s.Bankroll("u1"); got != defaultChips {
		t.Fatalf("expected the stack back in the bankroll, got %d", got)
	}
	if _, err := s.Get(tb.Code); err != nil {
		t.Fatalf("table should linger during the grace period, got %v", err)
	}
	if closed := s.ReapIdle(time.Now().Add(IdleGrace)); len(closed) != 1 {
		t.Fatalf("expected the empty table to close, got %v", closed)
	}
}

func TestNotifyDoesNotDeadlockWhenNotifierReadsBankroll(t *testing.T) {
	s := NewService(nil)
	// Mirrors the websocket broadcaster, which builds a per-viewer DTO and so
	// calls back into the service while the notification is being delivered.
	s.SetNotifier(func(_ string, tb *domainpoker.Table) {
		for _, p := range tb.Players {
			_ = s.Bankroll(p.ID)
		}
	})

	tb, err := s.CreateTable(6, 10)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := s.Join(tb.Code, "u1", 200); err != nil {
			t.Errorf("join: %v", err)
		}
		if err := s.Join(tb.Code, "u2", 200); err != nil {
			t.Errorf("second join: %v", err)
		}
		if err := s.StartHand(tb.Code, tb.Players[tb.NextStarterSeat()].ID); err != nil {
			t.Errorf("start: %v", err)
		}
		s.Leave(tb.Code, "u1")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlocked: notifier reading the bankroll blocked on the service mutex")
	}
}

func TestAwayPlayerIsSeatedOutAndRefunded(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 400); err != nil {
		t.Fatal(err)
	}
	if err := s.Join(tb.Code, "u2", 400); err != nil {
		t.Fatal(err)
	}

	s.MarkPresent(tb.Code, "u1")
	s.MarkPresent(tb.Code, "u2")
	s.MarkAway(tb.Code, "u1")

	start := time.Now()
	s.ReapIdle(start)
	if len(tb.Players) != 2 {
		t.Fatalf("player dropped before the away grace elapsed: %d left", len(tb.Players))
	}

	s.ReapIdle(start.Add(AwayGrace))
	if len(tb.Players) != 1 {
		t.Fatalf("expected the away player to be seated out, %d players remain", len(tb.Players))
	}
	if got := s.Bankroll("u1"); got != defaultChips {
		t.Fatalf("expected the stack refunded to the bankroll, got %d", got)
	}
}

func TestReconnectBeforeGraceKeepsSeat(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 400); err != nil {
		t.Fatal(err)
	}
	s.MarkPresent(tb.Code, "u1")
	s.MarkAway(tb.Code, "u1")
	s.MarkPresent(tb.Code, "u1")

	s.ReapIdle(time.Now().Add(AwayGrace * 2))
	if len(tb.Players) != 1 {
		t.Fatal("a reconnected player must keep their seat")
	}
}

func TestOnlyTheNextStarterCanDeal(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	for _, id := range []string{"u1", "u2", "u3"} {
		if err := s.Join(tb.Code, id, 400); err != nil {
			t.Fatal(err)
		}
	}
	starter := tb.NextStarterSeat()
	if starter < 0 {
		t.Fatal("expected a starter to be nominated")
	}
	wrong := tb.Players[(starter+1)%len(tb.Players)].ID
	if err := s.StartHand(tb.Code, wrong); err != ErrNotYourDeal {
		t.Fatalf("expected ErrNotYourDeal for a non-starter, got %v", err)
	}
	if err := s.StartHand(tb.Code, tb.Players[starter].ID); err != nil {
		t.Fatalf("starter could not deal: %v", err)
	}
}

func TestDealRightRotatesEachRound(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	for _, id := range []string{"u1", "u2", "u3"} {
		if err := s.Join(tb.Code, id, 4000); err != nil {
			t.Fatal(err)
		}
	}
	seen := make([]string, 0, 3)
	for round := 0; round < 3; round++ {
		starter := tb.NextStarterSeat()
		id := tb.Players[starter].ID
		seen = append(seen, id)
		if err := s.StartHand(tb.Code, id); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		for tb.Acting >= 0 {
			if err := s.Act(tb.Code, tb.Players[tb.Acting].ID, "fold", 0); err != nil {
				break
			}
		}
	}
	if seen[0] == seen[1] && seen[1] == seen[2] {
		t.Fatalf("the deal never rotated: %v", seen)
	}
}

func TestTurnTimeoutActsForThePlayer(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 400); err != nil {
		t.Fatal(err)
	}
	if err := s.Join(tb.Code, "u2", 400); err != nil {
		t.Fatal(err)
	}
	if err := s.StartHand(tb.Code, tb.Players[tb.NextStarterSeat()].ID); err != nil {
		t.Fatal(err)
	}
	acting := tb.Acting
	if acting < 0 {
		t.Fatal("expected someone to be on the clock")
	}
	if s.TurnRemaining(tb.Code) <= 0 {
		t.Fatal("expected a running shot clock")
	}

	start := time.Now()
	if touched := s.EnforceTurns(start); len(touched) != 0 {
		t.Fatalf("acted before the clock expired: %v", touched)
	}
	if touched := s.EnforceTurns(start.Add(TurnLimit)); len(touched) != 1 {
		t.Fatalf("expected the timeout to act, got %v", touched)
	}
	if tb.Acting == acting && !tb.Players[acting].Folded {
		t.Fatal("expected the timed-out player to be folded or the action to move on")
	}
}

func TestRejoinIsFreeAndReconnectKeepsSeat(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 1000); err != nil {
		t.Fatal(err)
	}
	if err := s.Join(tb.Code, "u2", 400); err != nil {
		t.Fatal(err)
	}
	if got := s.Bankroll("u1"); got != 0 {
		t.Fatalf("expected the whole bankroll on the table, got %d", got)
	}

	// App closed: the socket drops and the away clock starts.
	s.MarkPresent(tb.Code, "u1")
	s.MarkAway(tb.Code, "u1")

	// Reopened: rejoining with no bankroll left must still work and cost nothing.
	if err := s.Join(tb.Code, "u1", 0); err != nil {
		t.Fatalf("rejoining an existing seat should be free: %v", err)
	}
	if got := s.Bankroll("u1"); got != 0 {
		t.Fatalf("rejoining must not move chips, bankroll is %d", got)
	}
	if got := tb.Players[0].Stack; got != 1000 {
		t.Fatalf("stack should be untouched, got %d", got)
	}

	// Reconnecting the socket is what actually secures the seat; the rejoin
	// alone only buys another grace window to get that socket up.
	s.MarkPresent(tb.Code, "u1")
	s.MarkPresent(tb.Code, "u2")
	s.ReapIdle(time.Now().Add(AwayGrace * 2))
	if len(tb.Players) != 2 {
		t.Fatalf("a rejoined player was still seated out: %d players left", len(tb.Players))
	}
}

func TestSeatIsReclaimedWhenTheSocketNeverConnects(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	// Seated over HTTP, then the app dies before the websocket ever opens, so
	// MarkPresent/MarkAway never run for this player.
	if err := s.Join(tb.Code, "u1", 1000); err != nil {
		t.Fatal(err)
	}
	if got := s.Bankroll("u1"); got != 0 {
		t.Fatalf("expected the bankroll on the table, got %d", got)
	}

	s.ReapIdle(time.Now().Add(AwayGrace))

	if len(tb.Players) != 0 {
		t.Fatalf("the orphaned seat was never reclaimed: %d players remain", len(tb.Players))
	}
	if got := s.Bankroll("u1"); got != defaultChips {
		t.Fatalf("expected the stack refunded, got %d", got)
	}
}

func TestConnectedPlayerKeepsSeatIndefinitely(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 500); err != nil {
		t.Fatal(err)
	}
	s.MarkPresent(tb.Code, "u1")
	s.ReapIdle(time.Now().Add(AwayGrace * 10))
	if len(tb.Players) != 1 {
		t.Fatal("a connected player must not be seated out")
	}
}

func TestBonusGrantsChipsThenCoolsDown(t *testing.T) {
	s := NewService(nil)
	base := time.Now()
	s.now = func() time.Time { return base }

	before := s.Bankroll("u1")
	got, err := s.TopUp("u1")
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}
	if got != before+BonusChips {
		t.Fatalf("expected %d chips added, got %d from %d", BonusChips, got, before)
	}

	if _, err := s.TopUp("u1"); err != ErrBonusNotReady {
		t.Fatalf("a second immediate claim must be refused, got %v", err)
	}
	if left := s.BonusReadyIn("u1"); left <= 0 || left > BonusInterval {
		t.Fatalf("expected a countdown within the interval, got %v", left)
	}
}

func TestBonusIsClaimableAgainAfterTheInterval(t *testing.T) {
	s := NewService(nil)
	base := time.Now()
	s.now = func() time.Time { return base }
	if _, err := s.TopUp("u1"); err != nil {
		t.Fatal(err)
	}

	// One second short of the interval is still too early.
	s.now = func() time.Time { return base.Add(BonusInterval - time.Second) }
	if _, err := s.TopUp("u1"); err != ErrBonusNotReady {
		t.Fatalf("claim just before the interval must be refused, got %v", err)
	}

	s.now = func() time.Time { return base.Add(BonusInterval) }
	if left := s.BonusReadyIn("u1"); left != 0 {
		t.Fatalf("expected the bonus ready, %v remaining", left)
	}
	if _, err := s.TopUp("u1"); err != nil {
		t.Fatalf("claim after the interval should succeed: %v", err)
	}
}

func TestBonusIsClaimableWithChipsAlreadyInPlay(t *testing.T) {
	s := NewService(nil)
	tb, _ := s.CreateTable(6, 10)
	if err := s.Join(tb.Code, "u1", 1000); err != nil {
		t.Fatal(err)
	}
	// The old rule refused while any chips sat at a table; a timed bonus does not.
	if _, err := s.TopUp("u1"); err != nil {
		t.Fatalf("a seated player should still be able to claim: %v", err)
	}
	if got := s.Bankroll("u1"); got != BonusChips {
		t.Fatalf("expected %d in the bankroll, got %d", BonusChips, got)
	}
}

func TestBonusCooldownIsPerPlayer(t *testing.T) {
	s := NewService(nil)
	if _, err := s.TopUp("u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TopUp("u2"); err != nil {
		t.Fatalf("one player's claim must not block another: %v", err)
	}
}
