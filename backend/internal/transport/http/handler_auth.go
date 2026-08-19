package http

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	authapp "github.com/teamcutter/go-poker/internal/application/auth"
)

type AuthHandler struct {
	auth     *authapp.Service
	tokenTTL time.Duration
}

func NewAuthHandler(auth *authapp.Service, tokenTTL time.Duration) *AuthHandler {
	return &AuthHandler{auth: auth, tokenTTL: tokenTTL}
}

type telegramLoginRequest struct {
	InitData string `json:"init_data"`
}

type telegramLoginResponse struct {
	Token string  `json:"token"`
	User  UserDTO `json:"user"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req telegramLoginRequest
	if err := c.Bind(&req); err != nil {
		return fail(c, http.StatusBadRequest, "bad_request", "invalid request body")
	}
	if req.InitData == "" {
		return fail(c, http.StatusBadRequest, "bad_request", "init_data is required")
	}

	res, err := h.auth.Authenticate(c.Request().Context(), req.InitData, h.tokenTTL)
	if err != nil {
		return mapError(c, err)
	}

	return ok(c, http.StatusOK, telegramLoginResponse{
		Token: res.Token,
		User:  toUserDTO(res.User, res.PublicID),
	})
}

func (h *AuthHandler) Register(g *echo.Group) {
	g.POST("/auth/telegram", h.Login)
}
