package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"billing-app/backend/internal/models"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestSessionRoundTrip(t *testing.T) {
	m, err := NewSessionManager(testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	want := Identity{UserID: 7, Subject: "123", Email: "jane@example.com", Name: "Jane", Picture: "https://p/x.png"}

	token, expires := m.Issue(want, now)
	if !expires.Equal(now.Add(time.Hour)) {
		t.Fatalf("expires = %v, want %v", expires, now.Add(time.Hour))
	}
	got, err := m.Parse(token, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != want {
		t.Fatalf("identity = %+v, want %+v", got, want)
	}
}

func TestSessionRejectsExpiredTamperedAndForeign(t *testing.T) {
	m, _ := NewSessionManager(testSecret, time.Hour)
	other, _ := NewSessionManager(strings.Repeat("x", 32), time.Hour)
	now := time.Now()
	token, _ := m.Issue(Identity{Subject: "1", Email: "a@b.c", Name: "A"}, now)

	payload, sig, _ := strings.Cut(token, ".")
	cases := map[string]struct {
		token string
		at    time.Time
	}{
		"expired":          {token, now.Add(time.Hour)},
		"tampered payload": {payload + "x." + sig, now},
		"tampered sig":     {payload + "." + sig[:len(sig)-1] + "A", now},
		"no separator":     {payload, now},
		"empty":            {"", now},
	}
	for name, tc := range cases {
		if _, err := m.Parse(tc.token, tc.at); !errors.Is(err, ErrInvalidSession) {
			t.Errorf("%s: err = %v, want ErrInvalidSession", name, err)
		}
	}
	if _, err := other.Parse(token, now); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("token signed with another secret accepted: %v", err)
	}
}

func TestNewManagersValidateInput(t *testing.T) {
	if _, err := NewSessionManager("short", time.Hour); err == nil {
		t.Error("short secret accepted")
	}
	if _, err := NewSessionManager(testSecret, 0); err == nil {
		t.Error("zero ttl accepted")
	}
	if _, err := NewStateManager("short", time.Minute); err == nil {
		t.Error("short state secret accepted")
	}
}

func TestStateRoundTripAndRejections(t *testing.T) {
	m, err := NewStateManager(testSecret, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	pending, cookie, err := m.Issue(now)
	if err != nil {
		t.Fatal(err)
	}
	if pending.State == "" || pending.CodeVerifier == "" {
		t.Fatalf("empty pending login: %+v", pending)
	}

	got, err := m.Verify(cookie, pending.State, now.Add(time.Minute))
	if err != nil || got.CodeVerifier != pending.CodeVerifier {
		t.Fatalf("verify: got %+v, err %v", got, err)
	}

	other, _, _ := m.Issue(now)
	payload, sig, _ := strings.Cut(cookie, ".")
	cases := map[string]struct {
		cookie, state string
		at            time.Time
	}{
		"wrong state":     {cookie, other.State, now},
		"empty state":     {cookie, "", now},
		"expired":         {cookie, pending.State, now.Add(11 * time.Minute)},
		"no cookie":       {"", pending.State, now},
		"tampered cookie": {payload + "x." + sig, pending.State, now},
	}
	for name, tc := range cases {
		if _, err := m.Verify(tc.cookie, tc.state, tc.at); !errors.Is(err, ErrInvalidState) {
			t.Errorf("%s: err = %v, want ErrInvalidState", name, err)
		}
	}
}

// memStore is an in-memory authorized_users table.
type memStore struct {
	rows map[int64]*models.AuthorizedUser
}

func (s *memStore) FindBySubject(_ context.Context, sub string) (*models.AuthorizedUser, error) {
	for _, u := range s.rows {
		if u.GoogleSub == sub {
			return u, nil
		}
	}
	return nil, nil
}

func (s *memStore) FindByEmail(_ context.Context, email string) (*models.AuthorizedUser, error) {
	for _, u := range s.rows {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (s *memStore) BindSubject(_ context.Context, id int64, sub string) error {
	s.rows[id].GoogleSub = sub
	return nil
}

func (s *memStore) TouchLogin(_ context.Context, id int64) error {
	now := time.Now()
	s.rows[id].LastLoginAt = &now
	return nil
}

func TestAuthorizeBindsOnFirstLoginThenUsesGoogleID(t *testing.T) {
	store := &memStore{rows: map[int64]*models.AuthorizedUser{
		1: {ID: 1, Email: "owner@example.com", DisplayName: "Owner", IsActive: true},
		2: {ID: 2, Email: "former@example.com", GoogleSub: "g-former", IsActive: false},
	}}
	svc := &Service{Users: store}
	ctx := context.Background()

	// Unknown account: no row at all.
	if _, err := svc.Authorize(ctx, GoogleAccount{Subject: "g-x", Email: "stranger@example.com"}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("stranger: err = %v, want ErrNotAllowed", err)
	}

	// First login of the owner: matched by email, google id bound.
	id, err := svc.Authorize(ctx, GoogleAccount{Subject: "g-owner", Email: "owner@example.com", Name: "Google Name"})
	if err != nil {
		t.Fatal(err)
	}
	if id.UserID != 1 || id.Subject != "g-owner" || id.Name != "Owner" || store.rows[1].GoogleSub != "g-owner" {
		t.Fatalf("first login: %+v, row %+v", id, store.rows[1])
	}

	// Same Google account with a changed email still works (identity is the sub).
	if id, err = svc.Authorize(ctx, GoogleAccount{Subject: "g-owner", Email: "renamed@example.com"}); err != nil || id.UserID != 1 {
		t.Fatalf("returning login: %+v, %v", id, err)
	}

	// A different Google account claiming the owner's email is refused: the row is already bound.
	if _, err := svc.Authorize(ctx, GoogleAccount{Subject: "g-impostor", Email: "owner@example.com"}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("impostor: err = %v, want ErrNotAllowed", err)
	}

	// Deactivated rows are refused even with a bound sub.
	if _, err := svc.Authorize(ctx, GoogleAccount{Subject: "g-former", Email: "former@example.com"}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("deactivated: err = %v, want ErrNotAllowed", err)
	}

	ok, err := StillAuthorized(ctx, store, Identity{Subject: "g-owner"})
	if err != nil || !ok {
		t.Fatalf("owner should still be authorized: %v %v", ok, err)
	}
	if ok, _ := StillAuthorized(ctx, store, Identity{Subject: "g-former"}); ok {
		t.Fatal("deactivated account reported as authorized")
	}
}
