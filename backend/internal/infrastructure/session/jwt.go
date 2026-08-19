package session

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
)

type Manager struct {
	secret  []byte
	issuer  string
	ttl     time.Duration
	nowFunc func() time.Time
}

func NewManager(secret, issuer string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), issuer: issuer, ttl: ttl, nowFunc: time.Now}
}

type jwtClaims struct {
	// The published player id, so no database read is needed per request.
	PublicID string `json:"pid"`
	jwt.RegisteredClaims
}

func (m *Manager) Create(c domainsession.Claims) (string, error) {
	now := m.nowFunc()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		PublicID: c.PublicID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(c.UserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(c.ExpiresAt),
		},
	})
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign session: %w", err)
	}
	return signed, nil
}

func (m *Manager) Parse(raw string) (domainsession.Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domainsession.ErrInvalidSession
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return domainsession.Claims{}, domainsession.ErrInvalidSession
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return domainsession.Claims{}, domainsession.ErrInvalidSession
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return domainsession.Claims{}, domainsession.ErrInvalidSession
	}

	return domainsession.Claims{
		UserID:    userID,
		PublicID:  claims.PublicID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
