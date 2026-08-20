package poker

import "errors"

var (
	ErrTableFull      = errors.New("table is full")
	ErrAlreadySeated  = errors.New("already seated")
	ErrNotPlayersTurn = errors.New("not your turn")
	ErrHandNotStarted = errors.New("hand has not started")
	ErrPlayerFolded   = errors.New("player has folded")
	ErrInvalidAction  = errors.New("invalid action")
	ErrInvalidAmount  = errors.New("invalid amount")
	ErrNeedPlayers    = errors.New("need at least two players")
	ErrHandInProgress = errors.New("hand already in progress")
)

type Phase int

const (
	PhaseWaiting Phase = iota
	PhasePreflop
	PhaseFlop
	PhaseTurn
	PhaseRiver
	PhaseShowdown
	PhaseComplete
)

type Player struct {
	ID         string
	Seat       int
	Stack      int64
	Hole       [2]Card
	Folded     bool
	AllIn      bool
	StreetBet  int64
	HasActed   bool
	SittingOut bool
}

type Table struct {
	Code       string
	MaxSeats   int
	BigBlind   int64
	Players    []*Player
	Button     int
	Deck       *Deck
	Board      []Card
	Pot        int64
	Phase      Phase
	CurrentBet int64
	MinRaise   int64
	Acting     int
	LastAgg    int
	Winners    []int
	RandInt    func(n int) int
}

func (t *Table) randInt() func(n int) int {
	if t.RandInt != nil {
		return t.RandInt
	}
	return defaultRandInt
}

func NewTable(code string, maxSeats int, bigBlind int64) *Table {
	if maxSeats < 2 {
		maxSeats = 2
	}
	if maxSeats > 6 {
		maxSeats = 6
	}
	if bigBlind <= 0 {
		bigBlind = 10
	}
	return &Table{
		Code:       code,
		MaxSeats:   maxSeats,
		BigBlind:   bigBlind,
		Players:    make([]*Player, 0, maxSeats),
		Button:     -1,
		Phase:      PhaseWaiting,
		MinRaise:   bigBlind,
		CurrentBet: 0,
		Acting:     -1,
		LastAgg:    -1,
		RandInt:    defaultRandInt,
	}
}

func (t *Table) seatIndex(id string) int {
	for i, p := range t.Players {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func (t *Table) Sit(id string, stack int64) error {
	if t.seatIndex(id) >= 0 {
		return ErrAlreadySeated
	}
	if len(t.Players) >= t.MaxSeats {
		return ErrTableFull
	}
	if stack <= 0 {
		return ErrInvalidAmount
	}
	p := &Player{ID: id, Seat: len(t.Players), Stack: stack}
	// Arriving mid-hand means holding no cards in it, so the rest of the hand is
	// spent watching; the next StartHand clears both flags for any funded seat
	// and deals them in. Folded is what does the real work — the betting loops
	// skip on Folded || AllIn, and without it the newcomer would be handed the
	// action with an empty hand and stall the street.
	if t.Phase >= PhasePreflop && t.Phase < PhaseShowdown {
		p.SittingOut = true
		p.Folded = true
	}
	t.Players = append(t.Players, p)
	return nil
}

func (t *Table) Leave(id string) {
	i := t.seatIndex(id)
	if i < 0 {
		return
	}
	t.Players = append(t.Players[:i], t.Players[i+1:]...)
	for j := i; j < len(t.Players); j++ {
		t.Players[j].Seat = j
	}
	t.repairSeats()
}

func (t *Table) repairSeats() {
	n := len(t.Players)
	t.Winners = nil
	if n == 0 {
		t.Phase = PhaseWaiting
		t.Button = -1
		t.Acting = -1
		t.LastAgg = -1
		t.Board = nil
		t.Pot = 0
		return
	}
	if t.Button >= n {
		t.Button = n - 1
	}
	if t.LastAgg >= n {
		t.LastAgg = -1
	}
	if t.Phase < PhasePreflop || t.Phase >= PhaseShowdown {
		if t.Acting >= n {
			t.Acting = -1
		}
		return
	}
	if t.Acting >= n || t.Acting < 0 {
		t.Acting = 0
	}
	if len(t.inHand()) <= 1 {
		t.settleByFold(t.inHand())
		return
	}
	if !t.canAct() {
		t.endStreet()
		return
	}
	for t.Players[t.Acting].Folded || t.Players[t.Acting].AllIn {
		t.Acting = (t.Acting + 1) % n
	}
}

func (t *Table) inHand() []int {
	out := make([]int, 0)
	for i, p := range t.Players {
		if !p.Folded && p.Hole[0] != (Card{}) {
			out = append(out, i)
		}
	}
	return out
}

func (t *Table) activeSeats() []int {
	out := make([]int, 0, len(t.Players))
	for i, p := range t.Players {
		if p.Stack > 0 {
			out = append(out, i)
		}
	}
	return out
}

func (t *Table) nextActive(from int) int {
	n := len(t.Players)
	for k := 1; k <= n; k++ {
		idx := (from + k) % n
		if t.Players[idx].Stack > 0 {
			return idx
		}
	}
	return from
}

// NextStarterSeat is the seat that will take the button on the next hand, and
// therefore the only player allowed to deal it. Returns -1 when no hand can start.
func (t *Table) NextStarterSeat() int {
	if t.Phase >= PhasePreflop && t.Phase < PhaseShowdown {
		return -1
	}
	active := t.activeSeats()
	if len(active) < 2 {
		return -1
	}
	if t.Button < 0 {
		return active[0]
	}
	return t.nextActive(t.Button)
}

func (t *Table) AddChips(id string, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	i := t.seatIndex(id)
	if i < 0 {
		return ErrHandNotStarted
	}
	p := t.Players[i]
	// Anyone dealt into a live hand keeps the stack they started it with —
	// including an all-in player, whose stack is 0 but who can still win the
	// pot back. Only a player sitting the hand out may buy in mid-hand.
	if t.Phase >= PhasePreflop && t.Phase < PhaseShowdown && !p.SittingOut {
		return ErrInvalidAction
	}
	p.Stack += amount
	p.SittingOut = false
	return nil
}

func (t *Table) StartHand() error {
	if t.Phase >= PhasePreflop && t.Phase < PhaseShowdown {
		return ErrHandInProgress
	}
	active := t.activeSeats()
	if len(active) < 2 {
		return ErrNeedPlayers
	}
	if t.Button < 0 {
		t.Button = active[0]
	} else {
		t.Button = t.nextActive(t.Button)
	}
	t.Deck = NewDeck()
	t.Deck.Shuffle(t.randInt())
	t.Board = nil
	t.Pot = 0
	t.Winners = nil
	t.Phase = PhasePreflop
	t.CurrentBet = 0
	t.MinRaise = t.BigBlind
	t.Acting = -1
	t.LastAgg = -1
	for _, p := range t.Players {
		p.AllIn = false
		p.StreetBet = 0
		p.HasActed = false
		p.Hole[0] = Card{}
		p.Hole[1] = Card{}
		p.SittingOut = p.Stack <= 0
		p.Folded = p.SittingOut
		if !p.SittingOut {
			p.Hole[0], _ = t.Deck.Draw()
			p.Hole[1], _ = t.Deck.Draw()
		}
	}
	// Heads-up, the button posts the small blind and acts first preflop; with
	// three or more the small blind is the seat to the button's left.
	sb := t.Button
	if len(active) > 2 {
		sb = t.nextActive(t.Button)
	}
	bb := t.nextActive(sb)
	blind := t.BigBlind / 2
	if blind < 1 {
		blind = 1
	}
	t.forceBet(sb, blind)
	t.forceBet(bb, t.BigBlind)
	t.CurrentBet = t.BigBlind
	t.LastAgg = bb
	t.Acting = t.nextActive(bb)
	for k := 0; k < len(t.Players) && t.Players[t.Acting].AllIn; k++ {
		t.Acting = t.nextActive(t.Acting)
	}
	if !t.canAct() {
		t.endStreet()
	}
	return nil
}

func (t *Table) forceBet(seat int, amt int64) {
	p := t.Players[seat]
	if amt >= p.Stack {
		amt = p.Stack
		p.AllIn = true
	}
	p.StreetBet += amt
	p.Stack -= amt
	t.Pot += amt
	if p.Stack == 0 {
		p.AllIn = true
	}
}

func (t *Table) currentPlayer() *Player {
	if t.Acting < 0 || t.Acting >= len(t.Players) {
		return nil
	}
	return t.Players[t.Acting]
}

func (t *Table) toCall(seat int) int64 {
	p := t.Players[seat]
	d := t.CurrentBet - p.StreetBet
	if d < 0 {
		d = 0
	}
	if d > p.Stack {
		d = p.Stack
	}
	return d
}

func (t *Table) post(seat int, amt int64) {
	p := t.Players[seat]
	if amt > p.Stack {
		amt = p.Stack
	}
	p.StreetBet += amt
	p.Stack -= amt
	t.Pot += amt
	if p.Stack == 0 {
		p.AllIn = true
	}
}

func (t *Table) allActed() bool {
	for _, p := range t.Players {
		if p.Folded || p.AllIn {
			continue
		}
		if p.StreetBet < t.CurrentBet {
			return false
		}
		if !p.HasActed {
			return false
		}
	}
	return true
}

func (t *Table) endStreet() {
	for _, p := range t.Players {
		p.StreetBet = 0
		p.HasActed = false
	}
	t.CurrentBet = 0
	left := t.inHand()
	if len(left) <= 1 {
		t.settleByFold(left)
		return
	}
	switch t.Phase {
	case PhasePreflop:
		t.Phase = PhaseFlop
		t.dealBoard(3)
	case PhaseFlop:
		t.Phase = PhaseTurn
		t.dealBoard(1)
	case PhaseTurn:
		t.Phase = PhaseRiver
		t.dealBoard(1)
	case PhaseRiver:
		t.Phase = PhaseShowdown
		t.showdown(left)
		return
	}
	t.MinRaise = t.BigBlind
	t.LastAgg = -1
	if !t.canAct() {
		t.endStreet()
		return
	}
	// After the flop the action opens on the first live seat left of the
	// button, so the button always gets the last word on every street.
	t.Acting = (t.Button + 1) % len(t.Players)
	for t.Players[t.Acting].Folded || t.Players[t.Acting].AllIn {
		t.Acting = (t.Acting + 1) % len(t.Players)
	}
}

func (t *Table) canAct() bool {
	for _, p := range t.Players {
		if !p.Folded && !p.AllIn {
			return true
		}
	}
	return false
}

func (t *Table) dealBoard(n int) {
	for i := 0; i < n; i++ {
		c, _ := t.Deck.Draw()
		t.Board = append(t.Board, c)
	}
}

func (t *Table) settleByFold(left []int) {
	if len(left) == 0 {
		t.Phase = PhaseComplete
		return
	}
	if len(left) == 1 {
		i := left[0]
		t.payPot(i)
		return
	}
	t.endStreet()
}

func (t *Table) payPot(winner int) {
	p := t.Players[winner]
	p.Stack += t.Pot
	t.Pot = 0
	t.Winners = []int{winner}
	t.Phase = PhaseComplete
}

func (t *Table) showdown(in []int) {
	best := in[0]
	for _, i := range in[1:] {
		a := EvaluateAll(t.Players[i].Hole, t.Board)
		b := EvaluateAll(t.Players[best].Hole, t.Board)
		if better(a, b) {
			best = i
		}
	}
	winners := []int{best}
	for _, i := range in {
		if i == best {
			continue
		}
		a := EvaluateAll(t.Players[i].Hole, t.Board)
		b := EvaluateAll(t.Players[best].Hole, t.Board)
		if equalHands(a, b) {
			winners = append(winners, i)
		}
	}
	split := t.Pot / int64(len(winners))
	for _, i := range winners {
		t.Players[i].Stack += split
	}
	rem := t.Pot - split*int64(len(winners))
	if rem > 0 {
		t.Players[winners[0]].Stack += rem
	}
	t.Pot = 0
	t.Winners = winners
	t.Phase = PhaseComplete
}

func equalHands(a, b Hand) bool {
	if a.Cat != b.Cat {
		return false
	}
	for i := range a.Tiers {
		if a.Tiers[i] != b.Tiers[i] {
			return false
		}
	}
	return true
}

func (t *Table) Act(id string, action string, amount int64) error {
	i := t.seatIndex(id)
	if i < 0 {
		return errors.New("not seated")
	}
	if t.Phase < PhasePreflop {
		return ErrHandNotStarted
	}
	if t.Phase >= PhaseShowdown {
		return ErrHandNotStarted
	}
	p := t.Players[i]
	if p.Folded {
		return ErrPlayerFolded
	}
	if i != t.Acting {
		return ErrNotPlayersTurn
	}
	return t.handleAction(i, action, amount)
}

func (t *Table) handleAction(i int, action string, amount int64) error {
	p := t.Players[i]
	switch action {
	case "fold":
		p.Folded = true
		t.advanceToNext()
		return nil
	case "check":
		if t.toCall(i) != 0 {
			return ErrInvalidAction
		}
		p.HasActed = true
		t.advanceToNext()
		return nil
	case "call":
		if t.toCall(i) == 0 {
			p.HasActed = true
			t.advanceToNext()
			return nil
		}
		t.post(i, t.toCall(i))
		p.HasActed = true
		t.advanceToNext()
		return nil
	case "bet":
		if t.toCall(i) != 0 {
			return ErrInvalidAction
		}
		if amount < t.MinRaise {
			return ErrInvalidAmount
		}
		return t.raise(i, amount)
	case "raise":
		if amount <= t.CurrentBet {
			return ErrInvalidAmount
		}
		return t.raise(i, amount)
	case "allin":
		return t.allIn(i)
	}
	return ErrInvalidAction
}

// allIn commits every remaining chip.
//
// It deliberately does not route through raise. raise enforces the full
// min-raise increment, but a player whose entire stack falls short of that is
// still entitled to push it in — going through raise handed them
// ErrInvalidAmount and left them unable to act at all.
func (t *Table) allIn(i int) error {
	p := t.Players[i]
	if p.Stack <= 0 {
		return ErrInvalidAmount
	}
	// What this player will have wagered on this street once the stack is in.
	// Chips already committed count towards it, so the stack alone understates
	// the total and would leave an "all-in" player still holding chips.
	total := p.StreetBet + p.Stack
	p.HasActed = true
	if total > t.CurrentBet {
		// Only an all-in that covers a full raise reopens the betting and moves
		// the minimum. A short one simply becomes the amount to match.
		if total-t.CurrentBet >= t.MinRaise {
			t.MinRaise = total - t.CurrentBet
			t.LastAgg = i
		}
		t.CurrentBet = total
	}
	t.post(i, p.Stack)
	t.advanceToNext()
	return nil
}

func (t *Table) raise(i int, total int64) error {
	p := t.Players[i]
	if total <= t.CurrentBet {
		return ErrInvalidAmount
	}
	if total-t.CurrentBet < t.MinRaise {
		return ErrInvalidAmount
	}
	p.HasActed = true
	t.LastAgg = i
	t.MinRaise = total - t.CurrentBet
	t.post(i, total-p.StreetBet)
	t.CurrentBet = total
	t.advanceToNext()
	return nil
}

func (t *Table) advanceToNext() {
	n := len(t.Players)
	if len(t.inHand()) <= 1 {
		t.endStreet()
		return
	}
	if t.allActed() {
		t.endStreet()
		return
	}
	for k := 1; k <= n; k++ {
		idx := (t.Acting + k) % n
		if t.Players[idx].Folded || t.Players[idx].AllIn {
			continue
		}
		if t.Players[idx].StreetBet < t.CurrentBet || !t.Players[idx].HasActed {
			t.Acting = idx
			return
		}
	}
	t.endStreet()
}

func (t *Table) Spectate(id string) int {
	return t.seatIndex(id)
}
