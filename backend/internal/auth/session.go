package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const minSecretLen = 32

var ErrInvalidSession = errors.New("invalid session")

// signer seals a payload as base64(payload).base64(hmac-sha256) and opens it again,
// rejecting anything whose signature does not verify.
type signer struct {
	secret []byte
}

func (s signer) seal(payload []byte) string {
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + s.sign(encoded)
}

func (s signer) open(token string) ([]byte, error) {
	encoded, sig, ok := strings.Cut(token, ".")
	if !ok || encoded == "" || sig == "" {
		return nil, errors.New("malformed token")
	}
	if !constantTimeEqual(sig, s.sign(encoded)) {
		return nil, errors.New("bad signature")
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}

func (s signer) sign(encoded string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(encoded))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func constantTimeEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// SessionManager issues and validates stateless, HMAC-signed session tokens.
// The token carries the identity itself, so no server-side store is needed and
// every API replica can validate it with the shared secret.
type SessionManager struct {
	signer signer
	ttl    time.Duration
}

func NewSessionManager(secret string, ttl time.Duration) (*SessionManager, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("session secret must be at least %d characters", minSecretLen)
	}
	if ttl <= 0 {
		return nil, errors.New("session ttl must be positive")
	}
	return &SessionManager{signer: signer{secret: []byte(secret)}, ttl: ttl}, nil
}

func (m *SessionManager) TTL() time.Duration {
	return m.ttl
}

type sessionClaims struct {
	Identity
	IssuedAt  int64 `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// Issue returns a token for the identity and the instant it stops being valid.
func (m *SessionManager) Issue(id Identity, now time.Time) (string, time.Time) {
	expires := now.Add(m.ttl)
	payload, _ := json.Marshal(sessionClaims{Identity: id, IssuedAt: now.Unix(), ExpiresAt: expires.Unix()})
	return m.signer.seal(payload), expires
}

// Parse returns the identity inside a token, rejecting tampered or expired ones.
func (m *SessionManager) Parse(token string, now time.Time) (Identity, error) {
	payload, err := m.signer.open(token)
	if err != nil {
		return Identity{}, ErrInvalidSession
	}
	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Identity{}, ErrInvalidSession
	}
	if claims.Subject == "" || claims.Email == "" || now.Unix() >= claims.ExpiresAt {
		return Identity{}, ErrInvalidSession
	}
	return claims.Identity, nil
}
