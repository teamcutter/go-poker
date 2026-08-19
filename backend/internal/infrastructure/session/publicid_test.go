package session

import (
	"strconv"
	"strings"
	"testing"
	"time"

	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
)

func TestPublicIDNeverRevealsTheTelegramID(t *testing.T) {
	m := NewManager("sign-secret", "id-secret", "gopoker", time.Hour)
	const tgID int64 = 1111507794

	pub := m.PublicID(tgID)
	if pub == "" {
		t.Fatal("expected a derived identifier")
	}
	if strings.Contains(pub, strconv.FormatInt(tgID, 10)) {
		t.Fatalf("the telegram id leaked into the public id: %s", pub)
	}
	if pub == strconv.FormatInt(tgID, 10) {
		t.Fatal("public id is just the telegram id")
	}
}

func TestPublicIDIsStablePerUserAndDistinctBetweenUsers(t *testing.T) {
	m := NewManager("sign-secret", "id-secret", "gopoker", time.Hour)
	first, second := m.PublicID(42), m.PublicID(42)
	if first != second {
		t.Fatal("public id must be stable, or a player loses their seat identity")
	}
	if m.PublicID(42) == m.PublicID(43) {
		t.Fatal("two users collided on one public id")
	}
}

func TestPublicIDIsSecretDependent(t *testing.T) {
	a := NewManager("sign-secret", "id-secret-a", "gopoker", time.Hour)
	b := NewManager("sign-secret", "id-secret-b", "gopoker", time.Hour)
	if a.PublicID(42) == b.PublicID(42) {
		t.Fatal("public id must not be derivable without the public-id secret")
	}
}

func TestRotatingTheSigningSecretKeepsPlayerIdentity(t *testing.T) {
	before := NewManager("old-signing-secret", "id-secret", "gopoker", time.Hour)
	after := NewManager("new-signing-secret", "id-secret", "gopoker", time.Hour)

	if before.PublicID(42) != after.PublicID(42) {
		t.Fatal("rotating SESSION_SECRET must not change a player's identity")
	}

	// The signing keys really are different: a token from one must not verify
	// against the other, or the rotation achieved nothing.
	token, err := before.Create(domainsession.Claims{UserID: 42, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := after.Parse(token); err == nil {
		t.Fatal("a token signed with the old secret must stop verifying after rotation")
	}
}

func TestParsedClaimsCarryBothIdentifiers(t *testing.T) {
	m := NewManager("sign-secret", "id-secret", "gopoker", time.Hour)
	const tgID int64 = 987654321

	token, err := m.Create(domainsession.Claims{UserID: tgID, ExpiresAt: time.Now().Add(time.Hour)})
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
	if claims.PublicID != m.PublicID(tgID) {
		t.Fatal("parsed public id does not match the derivation")
	}
}
