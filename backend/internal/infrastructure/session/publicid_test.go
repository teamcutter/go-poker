package session

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	domainsession "github.com/teamcutter/go-poker/internal/domain/session"
)

func TestTokenCarriesOnlyThePublishedIdentifier(t *testing.T) {
	m := NewManager("sign-secret", "gopoker", time.Hour)
	const pub = "3f1c9a20-0b2e-4f77-9a1d-2c1f4e8b7a55"

	token, err := m.Create(domainsession.Claims{
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
	if claims.PublicID != pub {
		t.Fatalf("expected the published id to survive the round trip, got %q", claims.PublicID)
	}
}

// A JWT is signed, not encrypted: its payload is base64 that any holder can
// read. The token goes to the browser, so whatever is in it is published. This
// asserts on the wire format rather than on the parsed claims, because the leak
// would be in the bytes even if nothing on the server read the field back.
func TestTokenPayloadPublishesNothingButThePublicID(t *testing.T) {
	m := NewManager("sign-secret", "gopoker", time.Hour)
	const pub = "3f1c9a20-0b2e-4f77-9a1d-2c1f4e8b7a55"

	token, err := m.Create(domainsession.Claims{PublicID: pub, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected a three-part JWT, got %d parts", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if body["sub"] != pub {
		t.Fatalf("expected the subject to hold the published id, got %v", body["sub"])
	}
	// The whole point of the change: no identifier in here but the public one.
	for _, claim := range []string{"pid", "tg_id", "user_id"} {
		if _, ok := body[claim]; ok {
			t.Fatalf("%q must not be published: %s", claim, payload)
		}
	}
}

// A token naming nobody must be refused rather than accepted as an empty
// identity that every such holder would share.
func TestTokenWithoutAnyIdentityIsRejected(t *testing.T) {
	m := NewManager("sign-secret", "gopoker", time.Hour)

	anon := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "gopoker",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	raw, err := anon.SignedString([]byte("sign-secret"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.Parse(raw); err == nil {
		t.Fatal("a token carrying no identity must not verify")
	}
}

func TestRotatingTheSecretInvalidatesTokensButNotIdentity(t *testing.T) {
	before := NewManager("old-secret", "gopoker", time.Hour)
	after := NewManager("new-secret", "gopoker", time.Hour)
	const pub = "3f1c9a20-0b2e-4f77-9a1d-2c1f4e8b7a55"

	token, err := before.Create(domainsession.Claims{PublicID: pub, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := after.Parse(token); err == nil {
		t.Fatal("a token signed with the old secret must stop verifying after rotation")
	}
	// Identity lives in the database now, so rotation cannot disturb it.
	fresh, err := after.Create(domainsession.Claims{PublicID: pub, ExpiresAt: time.Now().Add(time.Hour)})
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
