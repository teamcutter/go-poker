package poker

import "testing"

func TestScenario20011HeadsUpAndThreePlayerBlinds(t *testing.T) {
	for _, seats := range []int{2, 3} {
		tb := NewTable("TEST", 3, 10)
		for i := 0; i < seats; i++ {
			if err := tb.Sit(string(rune('A'+i)), 200); err != nil {
				t.Fatal(err)
			}
		}
		if err := tb.StartHand(); err != nil {
			t.Fatal(err)
		}
		smallBlind := tb.Button
		if seats == 3 {
			smallBlind = (tb.Button + 1) % seats
		}
		bigBlind := (smallBlind + 1) % seats
		if tb.Players[smallBlind].StreetBet != 5 || tb.Players[bigBlind].StreetBet != 10 {
			t.Fatalf("%d players: wrong blinds, button=%d bets=%d,%d", seats, tb.Button, tb.Players[smallBlind].StreetBet, tb.Players[bigBlind].StreetBet)
		}
		wantActor := (bigBlind + 1) % seats
		if tb.Acting != wantActor {
			t.Fatalf("%d players: actor=%d, want %d", seats, tb.Acting, wantActor)
		}
	}
}

func TestScenario20013NextHandRotatesAndSkipsZeroStack(t *testing.T) {
	tb := NewTable("TEST", 3, 10)
	for _, id := range []string{"A", "B", "C"} {
		if err := tb.Sit(id, 200); err != nil {
			t.Fatal(err)
		}
	}
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	firstButton := tb.Button
	if err := tb.StartHand(); err != ErrHandInProgress {
		t.Fatalf("second start during a hand = %v, want ErrHandInProgress", err)
	}
	for tb.Phase != PhaseComplete {
		if err := tb.Act(tb.Players[tb.Acting].ID, "fold", 0); err != nil {
			t.Fatal(err)
		}
	}
	tb.Players[1].Stack = 0
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	if tb.Button == firstButton || tb.Button == 1 {
		t.Fatalf("button did not rotate to a funded seat: %d", tb.Button)
	}
	if !tb.Players[1].SittingOut || tb.Players[1].Hole != [2]Card{} {
		t.Fatalf("zero stack player was dealt in: %+v", tb.Players[1])
	}
	if tb.Phase != PhasePreflop || len(tb.Board) != 0 || len(tb.Winners) != 0 {
		t.Fatalf("new hand retained old state: phase=%v board=%v winners=%v", tb.Phase, tb.Board, tb.Winners)
	}
}
