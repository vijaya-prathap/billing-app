package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

// StateManager protects the redirect flow against CSRF and code injection. Before
// sending the browser to Google it issues a random state plus a PKCE verifier,
// wrapped in a signed, short-lived cookie value. On callback the state Google
// echoes back must match the one in the cookie, and the verifier is used to
// redeem the code. A stateless signed cookie means no server-side store and
// every API replica can validate it with the shared secret.
type StateManager struct {
	signer signer
	ttl    time.Duration
}

func NewStateManager(secret string, ttl time.Duration) (*StateManager, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("session secret must be at least %d characters", minSecretLen)
	}
	if ttl <= 0 {
		return nil, errors.New("state ttl must be positive")
	}
	return &StateManager{signer: signer{secret: []byte(secret)}, ttl: ttl}, nil
}

func (m *StateManager) TTL() time.Duration { return m.ttl }

// Pending is the in-flight login stored in the state cookie.
type Pending struct {
	State        string `json:"state"`
	CodeVerifier string `json:"verifier"`
	ExpiresAt    int64  `json:"exp"`
}

// Issue creates a new pending login and the cookie value that carries it.
func (m *StateManager) Issue(now time.Time) (Pending, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return Pending{}, "", err
	}
	p := Pending{
		State:        base64.RawURLEncoding.EncodeToString(buf),
		CodeVerifier: oauth2.GenerateVerifier(),
		ExpiresAt:    now.Add(m.ttl).Unix(),
	}
	payload, _ := json.Marshal(p)
	return p, m.signer.seal(payload), nil
}

// Verify checks the cookie value and that the state Google returned matches it.
func (m *StateManager) Verify(cookie, returnedState string, now time.Time) (Pending, error) {
	payload, err := m.signer.open(cookie)
	if err != nil {
		return Pending{}, ErrInvalidState
	}
	var p Pending
	if err := json.Unmarshal(payload, &p); err != nil || p.State == "" || p.CodeVerifier == "" {
		return Pending{}, ErrInvalidState
	}
	if now.Unix() >= p.ExpiresAt {
		return Pending{}, fmt.Errorf("%w: expired", ErrInvalidState)
	}
	if returnedState == "" || !constantTimeEqual(returnedState, p.State) {
		return Pending{}, fmt.Errorf("%w: state mismatch", ErrInvalidState)
	}
	return p, nil
}
