package session

import (
	"testing"
	"time"

	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
)

func TestTokenCarriesBothIdentifiers(t *testing.T) {
	m := NewManager("sign-secret", "gopoker", time.Hour)
	const tgID int64 = 987654321
	const pub = "3f1c9a20-0b2e-4f77-9a1d-2c1f4e8b7a55"

	token, err := m.Create(domainsession.Claims{
		UserID:    tgID,
		PublicID:  pub,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	claims, err := m.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != tgID {
		t.Fatalf("expected the telegram id for database use, got %d", claims.UserID)
	}
	if claims.PublicID != pub {
		t.Fatalf("expected the published id to survive the round trip, got %q", claims.PublicID)
	}
}

func TestRotatingTheSecretInvalidatesTokensButNotIdentity(t *testing.T) {
	before := NewManager("old-secret", "gopoker", time.Hour)
	after := NewManager("new-secret", "gopoker", time.Hour)
	const pub = "3f1c9a20-0b2e-4f77-9a1d-2c1f4e8b7a55"

	token, err := before.Create(domainsession.Claims{UserID: 42, PublicID: pub, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := after.Parse(token); err == nil {
		t.Fatal("a token signed with the old secret must stop verifying after rotation")
	}
	// Identity lives in the database now, so rotation cannot disturb it.
	fresh, err := after.Create(domainsession.Claims{UserID: 42, PublicID: pub, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := after.Parse(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if claims.PublicID != pub {
		t.Fatal("player identity must be unaffected by rotating the signing secret")
	}
}
