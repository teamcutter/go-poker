package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	pokerapp "github.com/teamcutter/go-poker/internal/application/poker"
	"github.com/teamcutter/go-poker/internal/domain/poker"
	"github.com/teamcutter/go-poker/internal/transport/http/middleware"
)

type PokerHandler struct {
	service *pokerapp.Service
}

func NewPokerHandler(service *pokerapp.Service) *PokerHandler {
	return &PokerHandler{service: service}
}

type createTableRequest struct {
	MaxSeats int   `json:"max_seats"`
	BigBlind int64 `json:"big_blind"`
}

type createTableResponse struct {
	Table TableDTO `json:"table"`
}

func (h *PokerHandler) Create(c echo.Context) error {
	var req createTableRequest
	if err := c.Bind(&req); err != nil {
		return fail(c, http.StatusBadRequest, "bad_request", "invalid request body")
	}
	tb, err := h.service.CreateTable(req.MaxSeats, req.BigBlind)
	if err != nil {
		return mapError(c, err)
	}
	return ok(c, http.StatusOK, createTableResponse{Table: toTableDTO(tb)})
}

func (h *PokerHandler) List(c echo.Context) error {
	tables := h.service.ListTables()
	out := make([]TableDTO, 0, len(tables))
	for _, tb := range tables {
		out = append(out, toTableDTO(tb))
	}
	return ok(c, http.StatusOK, out)
}

func (h *PokerHandler) Get(c echo.Context) error {
	code := c.Param("code")
	tb, err := h.service.Get(code)
	if err != nil {
		return mapError(c, err)
	}
	viewer := ""
	if claims, allowed := middleware.UserFrom(c); allowed {
		viewer = claims.PublicID
	}
	return ok(c, http.StatusOK, toTableDTOFor(tb, viewer, h.service.Bankroll(viewer), h.service.TurnRemaining(code)))
}

type joinRequest struct {
	BuyIn int64 `json:"buy_in"`
}

func (h *PokerHandler) Join(c echo.Context) error {
	code := c.Param("code")
	claims, allowed := middleware.UserFrom(c)
	if !allowed {
		return fail(c, http.StatusUnauthorized, "unauthorized", "missing session")
	}
	var req joinRequest
	if err := c.Bind(&req); err != nil {
		return fail(c, http.StatusBadRequest, "bad_request", "invalid request body")
	}
	userID := claims.PublicID
	if err := h.service.Join(code, userID, req.BuyIn); err != nil {
		return mapError(c, err)
	}
	return ok(c, http.StatusOK, map[string]any{
		"joined":   true,
		"bankroll": h.service.Bankroll(userID),
	})
}

func (h *PokerHandler) Leave(c echo.Context) error {
	code := c.Param("code")
	claims, allowed := middleware.UserFrom(c)
	if !allowed {
		return fail(c, http.StatusUnauthorized, "unauthorized", "missing session")
	}
	userID := claims.PublicID
	h.service.Leave(code, userID)
	return ok(c, http.StatusOK, map[string]any{
		"left":     true,
		"bankroll": h.service.Bankroll(userID),
	})
}

func (h *PokerHandler) Wallet(c echo.Context) error {
	claims, allowed := middleware.UserFrom(c)
	if !allowed {
		return fail(c, http.StatusUnauthorized, "unauthorized", "missing session")
	}
	return h.walletResponse(c, claims.PublicID)
}

func (h *PokerHandler) TopUp(c echo.Context) error {
	claims, allowed := middleware.UserFrom(c)
	if !allowed {
		return fail(c, http.StatusUnauthorized, "unauthorized", "missing session")
	}
	if _, err := h.service.TopUp(claims.PublicID); err != nil {
		return mapError(c, err)
	}
	return h.walletResponse(c, claims.PublicID)
}

func (h *PokerHandler) walletResponse(c echo.Context, userID string) error {
	return ok(c, http.StatusOK, map[string]any{
		"bankroll":          h.service.Bankroll(userID),
		"bonus_amount":      pokerapp.BonusChips,
		"bonus_ready_in_ms": h.service.BonusReadyIn(userID).Milliseconds(),
	})
}

func (h *PokerHandler) StartHand(c echo.Context) error {
	code := c.Param("code")
	claims, allowed := middleware.UserFrom(c)
	if !allowed {
		return fail(c, http.StatusUnauthorized, "unauthorized", "missing session")
	}
	if err := h.service.StartHand(code, claims.PublicID); err != nil {
		return mapError(c, err)
	}
	return ok(c, http.StatusOK, nil)
}

func (h *PokerHandler) Register(g *echo.Group, ws *WSHandler) {
	g.POST("/tables", h.Create)
	g.GET("/tables", h.List)
	g.GET("/tables/:code", h.Get)
	g.POST("/tables/:code/join", h.Join)
	g.POST("/tables/:code/leave", h.Leave)
	g.POST("/tables/:code/start", h.StartHand)
	g.GET("/poker/wallet", h.Wallet)
	g.POST("/poker/wallet/topup", h.TopUp)
}

type TableDTO map[string]any

func toTableDTO(tb *poker.Table) TableDTO {
	return toTableDTOFor(tb, "", 0, 0)
}

func toTableDTOFor(tb *poker.Table, viewerID string, bankroll int64, turnMsLeft int64) TableDTO {
	reveal := tb.Phase >= poker.PhaseShowdown
	players := make([]map[string]any, 0, len(tb.Players))
	for _, p := range tb.Players {
		var holes []string
		visible := reveal || viewerID != "" && p.ID == viewerID
		if visible {
			holes = cardsToStrings(p.Hole[:])
		} else {
			holes = []string{"", ""}
		}
		handName := ""
		handCards := []string{}
		if visible {
			if best, combo := poker.EvaluateBest(p.Hole, tb.Board); combo != nil {
				handName = best.Cat.String()
				handCards = cardsToStrings(combo)
			}
		}
		players = append(players, map[string]any{
			"hand_name":   handName,
			"hand_cards":  handCards,
			"id":          p.ID,
			"seat":        p.Seat,
			"stack":       p.Stack,
			"folded":      p.Folded,
			"all_in":      p.AllIn,
			"sitting_out": p.SittingOut,
			"street_bet":  p.StreetBet,
			"has_acted":   p.HasActed,
			"hole_cards":  holes,
		})
	}
	board := cardsToStrings(tb.Board)
	winners := make([]int, len(tb.Winners))
	copy(winners, tb.Winners)
	acting := tb.Acting
	if tb.Phase < poker.PhasePreflop || tb.Phase >= poker.PhaseShowdown {
		acting = -1
	}
	return map[string]any{
		"code":         tb.Code,
		"max_seats":    tb.MaxSeats,
		"big_blind":    tb.BigBlind,
		"players":      players,
		"board":        board,
		"pot":          tb.Pot,
		"phase":        phaseName(tb.Phase),
		"current_bet":  tb.CurrentBet,
		"min_raise":    tb.MinRaise,
		"acting":       acting,
		"button":       tb.Button,
		"winners":      winners,
		"bankroll":     bankroll,
		"starter":      tb.NextStarterSeat(),
		"turn_ms_left": turnMsLeft,
	}
}

func cardsToStrings(cards []poker.Card) []string {
	out := make([]string, 0, len(cards))
	for _, card := range cards {
		if card == (poker.Card{}) {
			out = append(out, "")
			continue
		}
		out = append(out, card.String())
	}
	return out
}

func phaseName(p poker.Phase) string {
	switch p {
	case poker.PhaseWaiting:
		return "waiting"
	case poker.PhasePreflop:
		return "preflop"
	case poker.PhaseFlop:
		return "flop"
	case poker.PhaseTurn:
		return "turn"
	case poker.PhaseRiver:
		return "river"
	case poker.PhaseShowdown:
		return "showdown"
	case poker.PhaseComplete:
		return "complete"
	}
	return "unknown"
}
