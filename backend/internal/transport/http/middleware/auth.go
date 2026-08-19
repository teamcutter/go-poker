package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
)

const ctxUserKey = "auth.claims"

func UserFrom(c echo.Context) (domainsession.Claims, bool) {
	claims, ok := c.Get(ctxUserKey).(domainsession.Claims)
	return claims, ok
}

type SessionParser interface {
	Parse(token string) (domainsession.Claims, error)
}

func Auth(sessions SessionParser) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}

			claims, err := sessions.Parse(token)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid session")
			}

			c.Set(ctxUserKey, claims)
			return next(c)
		}
	}
}
