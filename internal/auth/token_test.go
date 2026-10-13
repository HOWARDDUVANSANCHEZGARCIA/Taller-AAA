package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestManager(now *time.Time) *TokenManager {
	m := NewTokenManager([]byte("secreto-de-pruebas"), time.Hour)
	m.now = func() time.Time { return *now }
	return m
}

func TestTokenRoundTrip(t *testing.T) {
	now := time.Now()
	m := newTestManager(&now)

	token, expiresAt := m.Issue("user-1")
	if !expiresAt.After(now) {
		t.Error("la expiración debe estar en el futuro")
	}
	id, err := m.Verify(token)
	if err != nil || id != "user-1" {
		t.Fatalf("Verify = %q, %v; quería user-1, nil", id, err)
	}
}

func TestTokenExpires(t *testing.T) {
	now := time.Now()
	m := newTestManager(&now)
	token, _ := m.Issue("user-1")

	now = now.Add(time.Hour + time.Second)
	if _, err := m.Verify(token); !errors.Is(err, ErrExpiredToken) {
		t.Errorf("err = %v; quería ErrExpiredToken", err)
	}
}

func TestTokenRejectsTampering(t *testing.T) {
	now := time.Now()
	m := newTestManager(&now)
	token, _ := m.Issue("user-1")
	body, sig, _ := strings.Cut(token, ".")

	// Un payload distinto (otro usuario) con la firma original.
	other, _ := m.Issue("admin-1")
	otherBody, _, _ := strings.Cut(other, ".")

	cases := map[string]string{
		"vacío":           "",
		"sin firma":       body,
		"firma alterada":  body + "." + flipFirstChar(sig),
		"payload de otro": otherBody + "." + sig,
		"partes extra":    token + ".x",
		"basura":          "no-es-un-token",
		"firma con otro secret": func() string {
			o := NewTokenManager([]byte("otro"), time.Hour)
			o.now = m.now
			tk, _ := o.Issue("user-1")
			return tk
		}(),
	}
	for name, tk := range cases {
		if _, err := m.Verify(tk); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: err = %v; quería ErrInvalidToken", name, err)
		}
	}
}

// flipFirstChar cambia siempre el primer carácter, para alterar la firma de forma determinista.
func flipFirstChar(s string) string {
	if s[0] == 'A' {
		return "B" + s[1:]
	}
	return "A" + s[1:]
}
