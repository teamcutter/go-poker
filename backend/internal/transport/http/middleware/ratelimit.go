package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

func RateLimit(limiter Limiter, limit int, window time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.RealIP()
			if claims, ok := UserFrom(c); ok {
				key = "user:" + claims.PublicID
			}

			allowed, err := limiter.Allow(c.Request().Context(), key, limit, window)
			if err != nil {
				return next(c)
			}
			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"error": map[string]string{
						"code":    "rate_limited",
						"message": "too many requests, slow down",
					},
				})
			}
			return next(c)
		}
	}
}
