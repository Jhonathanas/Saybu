package authn

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	m, err := NewManager(strings.Repeat("a", 32), time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	token, expires, err := m.NewAccessToken("user-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || time.Until(expires) <= 0 {
		t.Fatalf("token/expires tidak valid: %q %v", token, expires)
	}

	claims, err := m.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.Role != "admin" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	a, _ := NewManager(strings.Repeat("a", 32), time.Minute)
	b, _ := NewManager(strings.Repeat("b", 32), time.Minute)

	token, _, err := a.NewAccessToken("user-1", "customer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestParseRejectsExpired(t *testing.T) {
	m, _ := NewManager(strings.Repeat("a", 32), 5*time.Millisecond)
	token, _, err := m.NewAccessToken("user-1", "customer")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := m.Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestNewManagerRejectsShortSecret(t *testing.T) {
	if _, err := NewManager("pendek", time.Minute); err == nil {
		t.Fatal("harus error untuk secret pendek")
	}
}

func TestRefreshToken(t *testing.T) {
	raw, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || hash == "" || raw == hash {
		t.Fatalf("raw=%q hash=%q", raw, hash)
	}
	if HashRefreshToken(raw) != hash {
		t.Fatal("hash tidak deterministik")
	}

	raw2, _, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw2 == raw {
		t.Fatal("refresh token duplikat")
	}
}
