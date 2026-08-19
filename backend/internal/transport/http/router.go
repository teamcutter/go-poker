package http

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/teamcutter/go-poker/internal/transport/http/middleware"
)

type Router struct {
	echo *echo.Echo

	auth  *AuthHandler
	poker *PokerHandler
	ws    *WSHandler

	sessions    middleware.SessionParser
	limiter     middleware.Limiter
	corsOrigins []string
	actionLimit int
	actionWin   time.Duration
	log         *zap.Logger
}

func NewRouter(
	log *zap.Logger,
	sessions middleware.SessionParser,
	auth *AuthHandler,
	poker *PokerHandler,
	ws *WSHandler,
	limiter middleware.Limiter,
	corsOrigins []string,
	actionLimit int,
	actionWindow time.Duration,
) *Router {
	return &Router{
		echo:        echo.New(),
		log:         log,
		sessions:    sessions,
		auth:        auth,
		poker:       poker,
		ws:          ws,
		limiter:     limiter,
		corsOrigins: corsOrigins,
		actionLimit: actionLimit,
		actionWin:   actionWindow,
	}
}

func (r *Router) Handler() http.Handler { return r.echo }

func (r *Router) Register() {
	e := r.echo
	e.HideBanner = true
	e.HidePort = true
	e.Use(echomw.Recover())
	e.Use(middleware.RequestLogger(r.log))
	// Only enabled when the frontend is served from another origin, e.g. the
	// SPA on Vercel talking to this API elsewhere. Empty means same-origin.
	if len(r.corsOrigins) > 0 {
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: r.corsOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
			AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAuthorization},
		}))
	}

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})

	public := e.Group("/api")
	r.auth.Register(public)
	if r.ws != nil {
		public.GET("/ws/tables/:code", r.ws.Connect)
	}

	api := e.Group("/api", middleware.Auth(r.sessions))
	if r.limiter != nil {
		api.Use(middleware.RateLimit(r.limiter, r.actionLimit, r.actionWin))
	}

	r.poker.Register(api, r.ws)
}
