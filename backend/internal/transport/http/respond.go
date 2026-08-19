package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/teamcutter/go-poker/internal/domain/session"
	"github.com/teamcutter/go-poker/internal/domain/store"
	domaintelegram "github.com/teamcutter/go-poker/internal/domain/telegram"

	authapp "github.com/teamcutter/go-poker/internal/application/auth"
	pokerapp "github.com/teamcutter/go-poker/internal/application/poker"

	domainpoker "github.com/teamcutter/go-poker/internal/domain/poker"
)

func ok(c echo.Context, status int, data any) error {
	return c.JSON(status, map[string]any{"data": data})
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func fail(c echo.Context, status int, code, message string) error {
	return c.JSON(status, map[string]any{"error": errorBody{Code: code, Message: message}})
}

func mapError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fail(c, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, store.ErrConflict):
		return fail(c, http.StatusConflict, "conflict", "conflicting state")
	case errors.Is(err, authapp.ErrInvalidInitData):
		return fail(c, http.StatusUnauthorized, "invalid_init_data", "invalid telegram init data")
	case errors.Is(err, domaintelegram.ErrExpiredInitData):
		return fail(c, http.StatusUnauthorized, "expired_init_data", "init data has expired")
	case errors.Is(err, session.ErrInvalidSession):
		return fail(c, http.StatusUnauthorized, "unauthorized", "invalid session")
	case errors.Is(err, pokerapp.ErrTableNotFound):
		return fail(c, http.StatusNotFound, "table_not_found", "table not found")
	case errors.Is(err, pokerapp.ErrNotEnoughChips):
		return fail(c, http.StatusConflict, "not_enough_chips", "not enough chips")
	case errors.Is(err, pokerapp.ErrNotYourDeal):
		return fail(c, http.StatusConflict, "not_your_deal", "it is another player's turn to deal")
	case errors.Is(err, pokerapp.ErrBonusNotReady):
		return fail(c, http.StatusConflict, "bonus_not_ready", "your next free chips are not ready yet")
	case errors.Is(err, domainpoker.ErrTableFull):
		return fail(c, http.StatusConflict, "table_full", "table is full")
	case errors.Is(err, domainpoker.ErrAlreadySeated):
		return fail(c, http.StatusConflict, "already_seated", "already seated")
	case errors.Is(err, domainpoker.ErrNotPlayersTurn):
		return fail(c, http.StatusConflict, "not_your_turn", "not your turn")
	case errors.Is(err, domainpoker.ErrHandNotStarted):
		return fail(c, http.StatusConflict, "hand_not_started", "hand not started")
	case errors.Is(err, domainpoker.ErrPlayerFolded):
		return fail(c, http.StatusConflict, "already_folded", "already folded")
	case errors.Is(err, domainpoker.ErrInvalidAction):
		return fail(c, http.StatusBadRequest, "invalid_action", "invalid action")
	case errors.Is(err, domainpoker.ErrInvalidAmount):
		return fail(c, http.StatusBadRequest, "invalid_amount", "invalid amount")
	case errors.Is(err, domainpoker.ErrNeedPlayers):
		return fail(c, http.StatusConflict, "need_players", "need at least two funded players")
	case errors.Is(err, domainpoker.ErrHandInProgress):
		return fail(c, http.StatusConflict, "hand_in_progress", "a hand is already in progress")
	default:
		return fail(c, http.StatusInternalServerError, "internal", "internal server error")
	}
}
