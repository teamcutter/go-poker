package poker

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"time"

	domainpoker "github.com/teamcutter/go-poker/internal/domain/poker"
)

const (
	defaultChips = 1000
	// BonusChips is granted per claim, once per BonusInterval.
	BonusChips = 500
	// How often a player may claim the free chip bonus.
	BonusInterval = 10 * time.Minute
	IdleGrace     = 30 * time.Second
	AwayGrace     = 30 * time.Second
	TurnLimit     = 30 * time.Second
	reapInterval  = 10 * time.Second
	turnInterval  = time.Second
)

var (
	ErrTableNotFound  = errors.New("table not found")
	ErrNotEnoughChips = errors.New("not enough chips")
	ErrBonusNotReady  = errors.New("free chips are not ready yet")
	ErrNotYourDeal    = errors.New("it is another player's turn to deal")
)

type turnState struct {
	seat     int
	phase    domainpoker.Phase
	deadline time.Time
}

type Notifier func(code string, table *domainpoker.Table)

type Service struct {
	mu         sync.Mutex
	tables     map[string]*domainpoker.Table
	emptySince map[string]time.Time
	present    map[string]int
	away       map[string]time.Time
	turns      map[string]turnState
	lastBonus  map[string]time.Time
	wallets    map[string]int64
	notifier   Notifier
	now        func() time.Time
}

func NewService(notifier Notifier) *Service {
	return &Service{
		tables:     make(map[string]*domainpoker.Table),
		emptySince: make(map[string]time.Time),
		present:    make(map[string]int),
		away:       make(map[string]time.Time),
		turns:      make(map[string]turnState),
		lastBonus:  make(map[string]time.Time),
		wallets:    make(map[string]int64),
		notifier:   notifier,
		now:        time.Now,
	}
}

func presenceKey(code, userID string) string {
	return code + "\x00" + userID
}

func splitPresenceKey(key string) (string, string, bool) {
	code, userID, found := strings.Cut(key, "\x00")
	return code, userID, found
}

func (s *Service) MarkPresent(code, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := presenceKey(code, userID)
	s.present[key]++
	delete(s.away, key)
}

// touchSeatLocked keeps a seat's liveness honest. Seating happens over HTTP but
// presence is only reported by the websocket, so a player whose socket never
// arrives would otherwise hold a seat — and the chips in it — indefinitely.
// Starting the away clock here means the seat is always reclaimable.
func (s *Service) touchSeatLocked(code, userID string) {
	key := presenceKey(code, userID)
	if s.present[key] > 0 {
		delete(s.away, key)
		return
	}
	s.away[key] = s.now()
}

func (s *Service) MarkAway(code, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := presenceKey(code, userID)
	if open := s.present[key]; open > 1 {
		s.present[key] = open - 1
		return
	}
	delete(s.present, key)
	s.away[key] = s.now()
}

func (s *Service) ReapIdle(now time.Time) []string {
	s.mu.Lock()
	touched := make([]string, 0)
	closed := make([]string, 0)
	// Deferred LIFO: the unlock below runs first, so the notifications are
	// always emitted with the mutex released. Broadcasting reads the bankroll,
	// which takes this same non-reentrant mutex.
	defer func() {
		for _, code := range touched {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	for key, since := range s.away {
		if now.Sub(since) < AwayGrace {
			continue
		}
		delete(s.away, key)
		code, userID, ok := splitPresenceKey(key)
		if !ok {
			continue
		}
		if s.seatOutLocked(code, userID) {
			touched = append(touched, code)
		}
	}

	for code, tb := range s.tables {
		if len(tb.Players) > 0 {
			delete(s.emptySince, code)
			continue
		}
		since, ok := s.emptySince[code]
		if !ok {
			s.emptySince[code] = now
			continue
		}
		if now.Sub(since) >= IdleGrace {
			delete(s.tables, code)
			delete(s.emptySince, code)
			delete(s.turns, code)
			closed = append(closed, code)
		}
	}
	return closed
}

func (s *Service) RunReaper(ctx context.Context) {
	reap := time.NewTicker(reapInterval)
	defer reap.Stop()
	turns := time.NewTicker(turnInterval)
	defer turns.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-reap.C:
			s.ReapIdle(s.now())
		case <-turns.C:
			s.EnforceTurns(s.now())
		}
	}
}

// syncTurnLocked restarts the shot clock whenever the action moves to a new
// seat, and clears it when no one is left to act.
func (s *Service) syncTurnLocked(code string, tb *domainpoker.Table) {
	if tb == nil || tb.Acting < 0 || tb.Acting >= len(tb.Players) ||
		tb.Phase < domainpoker.PhasePreflop || tb.Phase >= domainpoker.PhaseShowdown {
		delete(s.turns, code)
		return
	}
	// Phase matters too: the last player to act on one street can be the first
	// to act on the next, and that must still buy them a fresh clock.
	if cur, ok := s.turns[code]; ok && cur.seat == tb.Acting && cur.phase == tb.Phase {
		return
	}
	s.turns[code] = turnState{seat: tb.Acting, phase: tb.Phase, deadline: s.now().Add(TurnLimit)}
}

// TurnRemaining is milliseconds left on the acting player's clock, or 0 when
// no clock is running. Clients count down locally from this, so a phone whose
// clock disagrees with the server still shows the right number.
func (s *Service) TurnRemaining(code string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.turns[code]
	if !ok {
		return 0
	}
	left := st.deadline.Sub(s.now()).Milliseconds()
	if left < 0 {
		return 0
	}
	return left
}

// EnforceTurns folds (or checks, when it is free) for anyone who ran out of time.
func (s *Service) EnforceTurns(now time.Time) []string {
	s.mu.Lock()
	touched := make([]string, 0)
	defer func() {
		for _, code := range touched {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	for code, st := range s.turns {
		if now.Before(st.deadline) {
			continue
		}
		tb, ok := s.tables[code]
		if !ok {
			delete(s.turns, code)
			continue
		}
		if tb.Acting != st.seat || tb.Acting < 0 || tb.Acting >= len(tb.Players) {
			s.syncTurnLocked(code, tb)
			continue
		}
		id := tb.Players[tb.Acting].ID
		if err := tb.Act(id, "check", 0); err != nil {
			if err := tb.Act(id, "fold", 0); err != nil {
				delete(s.turns, code)
				continue
			}
		}
		s.syncTurnLocked(code, tb)
		touched = append(touched, code)
	}
	return touched
}

func (s *Service) SetNotifier(notifier Notifier) {
	s.notifier = notifier
}

func (s *Service) balance(id string) int64 {
	if b, ok := s.wallets[id]; ok {
		return b
	}
	return defaultChips
}

func (s *Service) Bankroll(id string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.balance(id)
}

func (s *Service) bonusReadyInLocked(id string) time.Duration {
	last, ok := s.lastBonus[id]
	if !ok {
		return 0
	}
	if elapsed := s.now().Sub(last); elapsed < BonusInterval {
		return BonusInterval - elapsed
	}
	return 0
}

// BonusReadyIn is how long until the player may claim again, zero when ready.
func (s *Service) BonusReadyIn(id string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bonusReadyInLocked(id)
}

func (s *Service) TopUp(id string) (int64, error) {
	s.mu.Lock()
	// The bankroll is part of every viewer's table state, so a claim has to be
	// pushed out or the client keeps rendering a stale figure.
	touched := make([]string, 0)
	defer func() {
		for _, code := range touched {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	if s.bonusReadyInLocked(id) > 0 {
		return 0, ErrBonusNotReady
	}
	s.wallets[id] = s.balance(id) + BonusChips
	s.lastBonus[id] = s.now()
	for code, tb := range s.tables {
		if tb.Spectate(id) >= 0 {
			touched = append(touched, code)
		}
	}
	return s.wallets[id], nil
}

func (s *Service) CreateTable(maxSeats int, bigBlind int64) (*domainpoker.Table, error) {
	code, err := newCode()
	if err != nil {
		return nil, err
	}
	tb := domainpoker.NewTable(code, maxSeats, bigBlind)
	s.mu.Lock()
	s.tables[code] = tb
	s.emptySince[code] = s.now()
	s.mu.Unlock()
	return tb, nil
}

func (s *Service) ListTables() []*domainpoker.Table {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domainpoker.Table, 0, len(s.tables))
	for _, tb := range s.tables {
		out = append(out, tb)
	}
	return out
}

func (s *Service) Get(code string) (*domainpoker.Table, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tb, ok := s.tables[code]
	if !ok {
		return nil, ErrTableNotFound
	}
	return tb, nil
}

func (s *Service) Join(code, userID string, buyIn int64) error {
	s.mu.Lock()
	changed := false
	defer func() {
		if changed {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	tb, ok := s.tables[code]
	if !ok {
		return ErrTableNotFound
	}
	seat := tb.Spectate(userID)
	if seat >= 0 && tb.Players[seat].Stack > 0 {
		s.touchSeatLocked(code, userID)
		return nil
	}
	bankroll := s.balance(userID)
	if buyIn <= 0 {
		buyIn = defaultChips
	}
	if buyIn > bankroll {
		buyIn = bankroll
	}
	if buyIn <= 0 {
		return ErrNotEnoughChips
	}
	if seat >= 0 {
		if err := tb.AddChips(userID, buyIn); err != nil {
			return err
		}
	} else if err := tb.Sit(userID, buyIn); err != nil {
		return err
	}
	s.wallets[userID] = bankroll - buyIn
	delete(s.emptySince, code)
	s.touchSeatLocked(code, userID)
	s.syncTurnLocked(code, tb)
	changed = true
	return nil
}

func (s *Service) seatOutLocked(code, userID string) bool {
	tb, ok := s.tables[code]
	if !ok {
		return false
	}
	if tb.Spectate(userID) < 0 {
		return false
	}
	for _, p := range tb.Players {
		if p.ID == userID {
			s.wallets[userID] = s.balance(userID) + p.Stack
			break
		}
	}
	tb.Leave(userID)
	delete(s.present, presenceKey(code, userID))
	s.syncTurnLocked(code, tb)
	if len(tb.Players) == 0 {
		s.emptySince[code] = s.now()
	}
	return true
}

func (s *Service) Leave(code, userID string) {
	s.mu.Lock()
	changed := false
	defer func() {
		if changed {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	delete(s.away, presenceKey(code, userID))
	changed = s.seatOutLocked(code, userID)
}

func (s *Service) StartHand(code, userID string) error {
	s.mu.Lock()
	changed := false
	defer func() {
		if changed {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	tb, ok := s.tables[code]
	if !ok {
		return ErrTableNotFound
	}
	starter := tb.NextStarterSeat()
	if starter < 0 || tb.Players[starter].ID != userID {
		return ErrNotYourDeal
	}
	if err := tb.StartHand(); err != nil {
		return err
	}
	s.syncTurnLocked(code, tb)
	changed = true
	return nil
}

func (s *Service) Act(code, userID, action string, amount int64) error {
	s.mu.Lock()
	changed := false
	defer func() {
		if changed {
			s.notify(code)
		}
	}()
	defer s.mu.Unlock()

	tb, ok := s.tables[code]
	if !ok {
		return ErrTableNotFound
	}
	if err := tb.Act(userID, action, amount); err != nil {
		return err
	}
	s.syncTurnLocked(code, tb)
	changed = true
	return nil
}

// notify must only ever be called with s.mu released.
func (s *Service) notify(code string) {
	if s.notifier == nil {
		return
	}
	s.mu.Lock()
	tb := s.tables[code]
	s.mu.Unlock()
	if tb != nil {
		s.notifier(code, tb)
	}
}

func newCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 4)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}
