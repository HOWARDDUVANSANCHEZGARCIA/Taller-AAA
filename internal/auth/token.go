package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("token inválido")
	ErrExpiredToken = errors.New("token expirado")
)

// TokenManager emite y verifica tokens firmados con HMAC-SHA256.
//
// Formato: base64url(claims).base64url(firma). Los claims solo llevan el ID
// del usuario y la expiración. El rol NO viaja en el token: se consulta al
// repositorio en cada solicitud, así el cliente no puede influir en él.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

type claims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

func NewTokenManager(secret []byte, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: secret, ttl: ttl, now: time.Now}
}

// Issue emite un token para el usuario y devuelve su instante de expiración.
func (m *TokenManager) Issue(userID string) (string, time.Time) {
	expiresAt := m.now().Add(m.ttl)
	payload, _ := json.Marshal(claims{Sub: userID, Exp: expiresAt.Unix()})
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + m.sign(body), expiresAt
}

// Verify valida firma y expiración y devuelve el ID de usuario del token.
func (m *TokenManager) Verify(token string) (string, error) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok || body == "" || sig == "" || strings.Contains(sig, ".") {
		return "", ErrInvalidToken
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", ErrInvalidToken
	}
	wantSig, _ := base64.RawURLEncoding.DecodeString(m.sign(body))
	if !hmac.Equal(gotSig, wantSig) {
		return "", ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", ErrInvalidToken
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil || c.Sub == "" {
		return "", ErrInvalidToken
	}
	if m.now().Unix() >= c.Exp {
		return "", ErrExpiredToken
	}
	return c.Sub, nil
}

func (m *TokenManager) sign(body string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
