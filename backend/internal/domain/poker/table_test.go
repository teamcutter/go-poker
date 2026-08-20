package poker

import "testing"

func sitAll(t *testing.T, tb *Table, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := tb.Sit(string(rune('A'+i)), 200); err != nil {
			t.Fatalf("sit %d: %v", i, err)
		}
	}
}

func TestTableSitLeave(t *testing.T) {
	tb := NewTable("TEST", 3, 10)
	sitAll(t, tb, 3)
	if err := tb.Sit("D", 200); err != ErrTableFull {
		t.Fatalf("expected full, got %v", err)
	}
	if err := tb.Sit("A", 200); err != ErrAlreadySeated {
		t.Fatalf("expected already seated, got %v", err)
	}
	tb.Leave("B")
	if err := tb.Sit("D", 200); err != nil {
		t.Fatalf("resit: %v", err)
	}
}

func TestStartHandDealsAndPostsBlinds(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhasePreflop {
		t.Fatalf("expected preflop, got %v", tb.Phase)
	}
	total := int64(0)
	for _, p := range tb.Players {
		if p.Hole[0] == (Card{}) || p.Hole[1] == (Card{}) {
			t.Fatalf("player %s missing hole cards", p.ID)
		}
		total += p.Stack
	}
	if total != 3*200-tb.Pot {
		t.Fatalf("chips lost: players=%d pot=%d", total, tb.Pot)
	}
	if tb.Pot != 15 {
		t.Fatalf("expected pot 15, got %d", tb.Pot)
	}
	if tb.CurrentBet != 10 {
		t.Fatalf("expected current bet 10, got %d", tb.CurrentBet)
	}
}

func TestFoldWinsPot(t *testing.T) {
	tb := NewTable("TEST", 3, 10)
	sitAll(t, tb, 3)
	tb.StartHand()
	first := tb.currentPlayer()
	tb.Act(first.ID, "fold", 0)
	second := tb.currentPlayer()
	if tb.Phase == PhaseComplete {
		t.Fatalf("hand ended too early")
	}
	tb.Act(second.ID, "fold", 0)
	if tb.Phase != PhaseComplete {
		t.Fatalf("expected complete after 2 folds, got %v", tb.Phase)
	}
	var last *Player
	for _, p := range tb.Players {
		if !p.Folded {
			last = p
		}
	}
	if last == nil {
		t.Fatal("no winner found")
	}
	if last.Stack != 205 {
		t.Fatalf("expected winner stack 205, got %d", last.Stack)
	}
	if tb.Winners == nil || len(tb.Winners) != 1 || tb.Players[tb.Winners[0]].ID != last.ID {
		t.Fatalf("wrong winner recorded")
	}
}

func TestAllCheckToShowdown(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.StartHand()
	for tb.Phase < PhaseShowdown {
		p := tb.currentPlayer()
		if p == nil {
			t.Fatal("no current player mid-hand")
		}
		if tb.CurrentBet > p.StreetBet {
			if err := tb.Act(p.ID, "call", 0); err != nil {
				t.Fatalf("call from %s: %v (phase=%v)", p.ID, err, tb.Phase)
			}
		} else {
			if err := tb.Act(p.ID, "check", 0); err != nil {
				t.Fatalf("check from %s: %v (phase=%v)", p.ID, err, tb.Phase)
			}
		}
	}
	if tb.Phase < PhaseShowdown {
		t.Fatalf("expected showdown/completed, got %v", tb.Phase)
	}
	if len(tb.Board) != 5 {
		t.Fatalf("expected 5 board cards, got %d", len(tb.Board))
	}
}

func TestShowdownSettlesPot(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.StartHand()
	for i := 0; i < 100 && tb.Phase < PhaseShowdown; i++ {
		p := tb.currentPlayer()
		if p == nil {
			break
		}
		if tb.CurrentBet > p.StreetBet {
			tb.Act(p.ID, "call", 0)
		} else {
			tb.Act(p.ID, "check", 0)
		}
	}
	if tb.Phase < PhaseShowdown {
		t.Fatalf("expected showdown/completed, got %v", tb.Phase)
	}
	total := int64(0)
	for _, p := range tb.Players {
		total += p.Stack
	}
	if total != 600 {
		t.Fatalf("chips not conserved: total=%d", total)
	}
}

func TestNextHandAfterCompletion(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.StartHand()
	for i := 0; i < 20; i++ {
		p := tb.currentPlayer()
		if p == nil || tb.Phase >= PhaseShowdown {
			break
		}
		if tb.CurrentBet > p.StreetBet {
			tb.Act(p.ID, "call", 0)
		} else {
			tb.Act(p.ID, "check", 0)
		}
	}
	if tb.Phase < PhaseShowdown {
		t.Fatalf("expected showdown/completed, got %v", tb.Phase)
	}
	first := tb.Players[0]
	if err := tb.Act(first.ID, "fold", 0); err != ErrHandNotStarted {
		t.Fatalf("action after showdown should be rejected, got %v", err)
	}
	if err := tb.StartHand(); err != nil {
		t.Fatalf("cannot start next hand: %v", err)
	}
	if tb.Phase != PhasePreflop {
		t.Fatalf("expected preflop, got %v", tb.Phase)
	}
	for _, p := range tb.Players {
		if p.Hole[0] == (Card{}) || p.Hole[1] == (Card{}) {
			t.Fatalf("player %s missing hole cards", p.ID)
		}
	}
}

func TestCallAndRaiseFlow(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.StartHand()
	first := tb.currentPlayer()
	if err := tb.Act(first.ID, "raise", 30); err != nil {
		t.Fatalf("raise: %v", err)
	}
	if tb.CurrentBet != 30 {
		t.Fatalf("expected current bet 30, got %d", tb.CurrentBet)
	}
	second := tb.currentPlayer()
	if second.ID == first.ID {
		t.Fatal("turn did not advance")
	}
	if err := tb.Act(second.ID, "call", 0); err != nil {
		t.Fatalf("call: %v", err)
	}
	if tb.Phase == PhaseFlop {
		t.Fatal("street ended before BB option")
	}
	third := tb.currentPlayer()
	if err := tb.Act(third.ID, "call", 0); err != nil {
		t.Fatalf("call: %v", err)
	}
	if tb.Phase != PhaseFlop {
		t.Fatalf("expected flop, got %v", tb.Phase)
	}
}

func TestShortStackAllInPreflop(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	tb.Sit("A", 5)
	tb.Sit("B", 200)
	tb.StartHand()
	a := tb.Players[0]
	if !a.AllIn {
		t.Fatalf("short stack should be all-in from blinds, stack=%d", a.Stack)
	}
	for i := 0; i < 100 && tb.Phase < PhaseShowdown; i++ {
		p := tb.currentPlayer()
		if p == nil {
			break
		}
		if tb.CurrentBet > p.StreetBet {
			tb.Act(p.ID, "call", 0)
		} else {
			tb.Act(p.ID, "check", 0)
		}
	}
	if tb.Phase < PhaseShowdown {
		t.Fatalf("expected auto-run to showdown, got %v", tb.Phase)
	}
	total := int64(0)
	for _, p := range tb.Players {
		total += p.Stack
	}
	if total != 205 {
		t.Fatalf("chips not conserved: total=%d", total)
	}
}

func TestInvalidActions(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	if err := tb.Act("A", "fold", 0); err != ErrHandNotStarted {
		t.Fatalf("expected hand not started, got %v", err)
	}
	tb.StartHand()
	if err := tb.Act("B", "fold", 0); err != ErrNotPlayersTurn {
		t.Fatalf("expected not your turn, got %v", err)
	}
	p := tb.currentPlayer()
	if err := tb.Act(p.ID, "check", 0); err != ErrInvalidAction {
		t.Fatalf("expected invalid action on check facing bet, got %v", err)
	}
}

func TestLeaveDuringGame(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.StartHand()
	tb.Leave("B")
	if len(tb.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(tb.Players))
	}
}

func TestStartHandShufflesDeck(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		tb := NewTable("TEST", 6, 10)
		sitAll(t, tb, 3)
		if err := tb.StartHand(); err != nil {
			t.Fatal(err)
		}
		key := ""
		for _, p := range tb.Players {
			key += p.Hole[0].String() + p.Hole[1].String()
		}
		seen[key] = true
	}
	if len(seen) < 2 {
		t.Fatalf("deck is not shuffled: %d distinct deals across 20 hands", len(seen))
	}
}

func TestHoleCardsSurviveForRevealAndHandRestarts(t *testing.T) {
	tb := NewTable("TEST", 3, 10)
	sitAll(t, tb, 2)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	p := tb.currentPlayer()
	if err := tb.Act(p.ID, "fold", 0); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseComplete {
		t.Fatalf("expected complete, got %v", tb.Phase)
	}
	for _, pl := range tb.Players {
		if pl.Hole[0] == (Card{}) || pl.Hole[1] == (Card{}) {
			t.Fatalf("player %s hole cards cleared before reveal", pl.ID)
		}
	}
	if err := tb.StartHand(); err != nil {
		t.Fatalf("restart after complete: %v", err)
	}
	if tb.Phase != PhasePreflop {
		t.Fatalf("expected preflop after restart, got %v", tb.Phase)
	}
}

func TestZeroStackPlayerSitsOut(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.Players[2].Stack = 0
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	if !tb.Players[2].SittingOut {
		t.Fatal("expected the broke player to sit out")
	}
	if tb.Players[2].Hole[0] != (Card{}) {
		t.Fatal("sitting-out player was dealt cards")
	}
	if tb.Acting == 2 {
		t.Fatal("sitting-out player must never be given the action")
	}
	for _, i := range []int{0, 1} {
		if tb.Players[i].Hole[0] == (Card{}) {
			t.Fatalf("funded player %d was not dealt", i)
		}
	}
}

func TestStartHandNeedsTwoFundedPlayers(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.Players[1].Stack = 0
	tb.Players[2].Stack = 0
	if err := tb.StartHand(); err != ErrNeedPlayers {
		t.Fatalf("expected ErrNeedPlayers, got %v", err)
	}
}

func TestAddChipsReturnsPlayerToTheGame(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 2)
	tb.Players[1].Stack = 0
	if err := tb.StartHand(); err != ErrNeedPlayers {
		t.Fatalf("expected ErrNeedPlayers, got %v", err)
	}
	if err := tb.AddChips("B", 500); err != nil {
		t.Fatalf("add chips: %v", err)
	}
	if err := tb.StartHand(); err != nil {
		t.Fatalf("start after rebuy: %v", err)
	}
	if tb.Players[1].SittingOut {
		t.Fatal("player should be dealt in after rebuying")
	}
}

func TestLeaveMidHandKeepsIndexesValid(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 4)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	tb.Button = 3
	tb.Leave("D")
	tb.Leave("C")
	if tb.Button >= len(tb.Players) {
		t.Fatalf("button %d out of range for %d players", tb.Button, len(tb.Players))
	}
	if tb.Acting >= len(tb.Players) {
		t.Fatalf("acting %d out of range for %d players", tb.Acting, len(tb.Players))
	}
	acting := tb.currentPlayer()
	if acting == nil {
		t.Fatal("no acting player left after the leaves")
	}
	if err := tb.Act(acting.ID, "fold", 0); err != nil {
		t.Fatalf("act after leave: %v", err)
	}
	if tb.Phase != PhaseComplete {
		t.Fatalf("expected the hand to settle heads-up, got %v", tb.Phase)
	}
}

func TestLeaveEmptiesTableCleanly(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 2)
	tb.StartHand()
	tb.Leave("A")
	tb.Leave("B")
	if len(tb.Players) != 0 {
		t.Fatalf("expected an empty table, got %d players", len(tb.Players))
	}
	if tb.Phase != PhaseWaiting || tb.Acting != -1 || tb.Button != -1 {
		t.Fatalf("expected a reset table, got phase=%v acting=%d button=%d", tb.Phase, tb.Acting, tb.Button)
	}
}

func TestAllInPlayerCannotRebuyMidHand(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	p := tb.currentPlayer()
	if err := tb.Act(p.ID, "allin", 0); err != nil {
		t.Fatalf("allin: %v", err)
	}
	if p.Stack != 0 || !p.AllIn {
		t.Fatalf("expected an all-in player with an empty stack, got stack=%d allin=%v", p.Stack, p.AllIn)
	}
	if tb.Phase >= PhaseShowdown {
		t.Skip("hand resolved immediately; nothing to guard")
	}
	if err := tb.AddChips(p.ID, 500); err != ErrInvalidAction {
		t.Fatalf("expected a mid-hand rebuy to be refused, got %v", err)
	}
	if p.Stack != 0 {
		t.Fatalf("stack changed mid-hand: %d", p.Stack)
	}
}

func TestSittingOutPlayerMayRebuyMidHand(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	tb.Players[2].Stack = 0
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	if !tb.Players[2].SittingOut {
		t.Fatal("expected the broke player to be sitting out")
	}
	if err := tb.AddChips("C", 500); err != nil {
		t.Fatalf("a sitting-out player should be able to buy back in: %v", err)
	}
	if tb.Players[2].SittingOut {
		t.Fatal("expected the player to be back in for the next hand")
	}
}

func TestPostflopActionOpensLeftOfButton(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6 && tb.Phase == PhasePreflop; i++ {
		p := tb.currentPlayer()
		if err := tb.Act(p.ID, "call", 0); err != nil {
			t.Fatalf("call: %v", err)
		}
	}
	if tb.Phase != PhaseFlop {
		t.Fatalf("expected the flop, got %v", tb.Phase)
	}
	want := (tb.Button + 1) % len(tb.Players)
	if tb.Acting != want {
		t.Fatalf("postflop must open left of the button: want seat %d, got %d (button %d)", want, tb.Acting, tb.Button)
	}
}

func TestSingleCheckDoesNotEndTheStreet(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 3)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6 && tb.Phase == PhasePreflop; i++ {
		tb.Act(tb.currentPlayer().ID, "call", 0)
	}
	for seat := 0; seat < 2; seat++ {
		p := tb.currentPlayer()
		if err := tb.Act(p.ID, "check", 0); err != nil {
			t.Fatalf("check %d: %v", seat, err)
		}
		if tb.Phase != PhaseFlop {
			t.Fatalf("street ended after %d of 3 checks: phase=%v", seat+1, tb.Phase)
		}
	}
	if err := tb.Act(tb.currentPlayer().ID, "check", 0); err != nil {
		t.Fatal(err)
	}
	if tb.Phase != PhaseTurn {
		t.Fatalf("expected the turn once everyone checked, got %v", tb.Phase)
	}
}

func TestHeadsUpButtonPostsSmallBlindAndActsFirst(t *testing.T) {
	tb := NewTable("TEST", 2, 10)
	sitAll(t, tb, 2)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	button := tb.Players[tb.Button]
	other := tb.Players[(tb.Button+1)%2]
	if button.StreetBet != tb.BigBlind/2 {
		t.Fatalf("heads-up button posts the small blind, posted %d", button.StreetBet)
	}
	if other.StreetBet != tb.BigBlind {
		t.Fatalf("heads-up non-button posts the big blind, posted %d", other.StreetBet)
	}
	if tb.Acting != tb.Button {
		t.Fatalf("heads-up button acts first preflop: acting=%d button=%d", tb.Acting, tb.Button)
	}
}

// Regression: a stack smaller than one minimum raise used to be rejected by
// raise, leaving the player unable to act at all — the client showed "All-in 5"
// and the server answered ErrInvalidAmount.
func TestAllInBelowMinRaiseIsLegal(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 2)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	// A fresh street with no bet yet, and the player to act holding less than
	// the minimum raise.
	tb.CurrentBet = 0
	tb.MinRaise = 10
	for _, pl := range tb.Players {
		pl.StreetBet = 0
		pl.HasActed = false
	}
	p := tb.Players[tb.Acting]
	p.Stack = 5

	if err := tb.Act(p.ID, "allin", 0); err != nil {
		t.Fatalf("a short all-in must be legal, got %v", err)
	}
	if p.Stack != 0 || !p.AllIn {
		t.Fatalf("expected an empty stack marked all-in, got stack=%d allin=%v", p.Stack, p.AllIn)
	}
	if tb.CurrentBet != 5 {
		t.Fatalf("expected the all-in to become the bet to match, got %d", tb.CurrentBet)
	}
	if tb.MinRaise != 10 {
		t.Fatalf("a short all-in must not lower the minimum raise, got %d", tb.MinRaise)
	}
}

// Regression: allin passed the stack where raise expects the total to raise to,
// so chips already committed on the street were double-counted and the player
// was left holding chips after supposedly going all-in.
func TestAllInIncludesChipsAlreadyCommitted(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 2)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	tb.CurrentBet = 20
	tb.MinRaise = 10
	for _, pl := range tb.Players {
		pl.StreetBet = 0
		pl.HasActed = false
	}
	p := tb.Players[tb.Acting]
	p.StreetBet = 10
	p.Stack = 30

	if err := tb.Act(p.ID, "allin", 0); err != nil {
		t.Fatalf("allin: %v", err)
	}
	if p.Stack != 0 || !p.AllIn {
		t.Fatalf("all-in must commit the whole stack, got stack=%d allin=%v", p.Stack, p.AllIn)
	}
	if p.StreetBet != 40 {
		t.Fatalf("expected 40 committed (10 already in plus 30), got %d", p.StreetBet)
	}
	if tb.CurrentBet != 40 {
		t.Fatalf("expected the current bet to rise to 40, got %d", tb.CurrentBet)
	}
}

// Someone who sits down while a hand is running has no cards in it, so they
// must watch rather than be dealt the action — previously they were handed the
// turn with an empty hand and the street could not complete.
func TestSitMidHandSpectatesUntilTheNextDeal(t *testing.T) {
	tb := NewTable("TEST", 6, 10)
	sitAll(t, tb, 2)
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	if err := tb.Sit("Z", 200); err != nil {
		t.Fatalf("sit: %v", err)
	}
	z := tb.Players[tb.seatIndex("Z")]
	if !z.SittingOut {
		t.Fatal("a mid-hand arrival should be sitting out")
	}
	if z.Hole[0] != (Card{}) {
		t.Fatal("a mid-hand arrival must not hold cards")
	}

	for i := 0; i < 100 && tb.Phase < PhaseShowdown; i++ {
		p := tb.currentPlayer()
		if p == nil {
			break
		}
		if p.ID == "Z" {
			t.Fatal("the spectator was given the turn")
		}
		if tb.CurrentBet > p.StreetBet {
			tb.Act(p.ID, "call", 0)
		} else {
			tb.Act(p.ID, "check", 0)
		}
	}
	if tb.Phase < PhaseShowdown {
		t.Fatalf("hand stalled at %v — the spectator blocked the street", tb.Phase)
	}

	// The wait is one hand long: the next deal brings them in.
	if err := tb.StartHand(); err != nil {
		t.Fatalf("start next hand: %v", err)
	}
	if z.SittingOut || z.Folded {
		t.Fatalf("expected to be dealt in, got sittingOut=%v folded=%v", z.SittingOut, z.Folded)
	}
	if z.Hole[0] == (Card{}) {
		t.Fatal("expected hole cards on the next hand")
	}
}
