package session

import (
	"errors"
	"time"
)

var ErrInvalidSession = errors.New("invalid session")

type Claims struct {
	UserID    int64
	PublicID  string
	ExpiresAt time.Time
}

func (c Claims) Expired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

type Manager interface {
	Create(claims Claims) (string, error)
	Parse(token string) (Claims, error)
	PublicID(userID int64) string
}
