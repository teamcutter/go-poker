package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
)

type Manager struct {
	secret         []byte
	publicIDSecret []byte
	issuer         string
	ttl            time.Duration
	nowFunc        func() time.Time
}

// publicIDSecret is deliberately separate from secret: signing keys should be
// rotatable at will, but rotating the key that derives player identity would
// orphan anything ever stored against a player id.
func NewManager(secret, publicIDSecret, issuer string, ttl time.Duration) *Manager {
	return &Manager{
		secret:         []byte(secret),
		publicIDSecret: []byte(publicIDSecret),
		issuer:         issuer,
		ttl:            ttl,
		nowFunc:        time.Now,
	}
}

type jwtClaims struct {
	jwt.RegisteredClaims
}

// PublicID is the only identifier ever published. It is derived from the
// Telegram id so it stays stable per user, but is opaque to other players —
// broadcasting the raw Telegram id would hand every opponent a permanent,
// real-world handle for the account. Domain-separated so it can never collide
// with another use of the same secret.
func (m *Manager) PublicID(userID int64) string {
	mac := hmac.New(sha256.New, m.publicIDSecret)
	mac.Write([]byte("gopoker:public-id:v1:"))
	mac.Write([]byte(strconv.FormatInt(userID, 10)))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func (m *Manager) Create(c domainsession.Claims) (string, error) {
	now := m.nowFunc()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
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
		PublicID:  m.PublicID(userID),
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
