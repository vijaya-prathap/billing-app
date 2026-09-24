// Package auth implements Google sign-in with the OAuth 2.0 authorization code flow.
//
// Google authenticates the person; this package decides whether that person is
// authorized (a row in authorized_users) and issues the signed session cookie
// that the API middleware checks on every request. Google passwords never reach
// the application, and the client secret never leaves the server.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"billing-app/backend/internal/models"
)

// Identity is what a session carries about the signed-in person.
type Identity struct {
	// UserID is the authorized_users row; Subject is Google's stable account ID.
	UserID  int64  `json:"uid"`
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
}

var (
	// ErrInvalidToken means the callback could not be turned into a verified Google account.
	ErrInvalidToken = errors.New("invalid id token")
	// ErrNotAllowed means the Google account is genuine but not in authorized_users.
	ErrNotAllowed = errors.New("account not authorized")
	// ErrInvalidState means the OAuth state was missing, tampered with or expired.
	ErrInvalidState = errors.New("invalid oauth state")
)

// GoogleAccount is what Google tells us about a person after they sign in.
type GoogleAccount struct {
	Subject string
	Email   string
	Name    string
	Picture string
}

// Exchanger drives the redirect flow against Google. The production implementation
// is GoogleOAuth; tests substitute a fake so they never touch the network.
type Exchanger interface {
	// AuthCodeURL is where the browser is sent to sign in with Google.
	AuthCodeURL(state, codeVerifier string) string
	// Exchange turns the callback's authorization code into a verified account.
	Exchange(ctx context.Context, code, codeVerifier string) (GoogleAccount, error)
}

// UserStore is the authorized_users table. Lookups return (nil, nil) when nothing matches.
type UserStore interface {
	FindBySubject(ctx context.Context, sub string) (*models.AuthorizedUser, error)
	FindByEmail(ctx context.Context, email string) (*models.AuthorizedUser, error)
	BindSubject(ctx context.Context, id int64, sub string) error
	TouchLogin(ctx context.Context, id int64) error
}

// Service ties the pieces together for the HTTP layer.
type Service struct {
	OAuth        Exchanger
	Sessions     *SessionManager
	States       *StateManager
	Users        UserStore
	CookieSecure bool
}

// Authorize decides whether a Google account may use the application.
//
// The Google account ID (sub) is the identity. Because an administrator cannot know
// someone's sub before they have signed in, a row whose sub is still empty is matched
// by verified email once and the sub is bound to it. From then on only the sub counts,
// so a row can never be claimed twice and an email change at Google is harmless.
func (s *Service) Authorize(ctx context.Context, acct GoogleAccount) (Identity, error) {
	user, err := s.Users.FindBySubject(ctx, acct.Subject)
	if err != nil {
		return Identity{}, fmt.Errorf("look up user by google id: %w", err)
	}
	if user == nil {
		candidate, err := s.Users.FindByEmail(ctx, acct.Email)
		if err != nil {
			return Identity{}, fmt.Errorf("look up user by email: %w", err)
		}
		if candidate == nil || candidate.GoogleSub != "" || !candidate.IsActive {
			return Identity{}, fmt.Errorf("%w: %s", ErrNotAllowed, acct.Email)
		}
		if err := s.Users.BindSubject(ctx, candidate.ID, acct.Subject); err != nil {
			return Identity{}, fmt.Errorf("bind google id to user %d: %w", candidate.ID, err)
		}
		user = candidate
	} else if !user.IsActive {
		return Identity{}, fmt.Errorf("%w: %s (deactivated)", ErrNotAllowed, acct.Email)
	}
	if err := s.Users.TouchLogin(ctx, user.ID); err != nil {
		return Identity{}, fmt.Errorf("record login for user %d: %w", user.ID, err)
	}

	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(acct.Name)
	}
	if name == "" {
		name = acct.Email
	}
	return Identity{
		UserID:  user.ID,
		Subject: acct.Subject,
		Email:   strings.ToLower(acct.Email),
		Name:    name,
		Picture: acct.Picture,
	}, nil
}

// StillAuthorized is the per-request re-check behind the middleware: a session
// stays valid only while its Google account remains an active authorized user.
func StillAuthorized(ctx context.Context, users UserStore, id Identity) (bool, error) {
	user, err := users.FindBySubject(ctx, id.Subject)
	if err != nil {
		return false, err
	}
	return user != nil && user.IsActive, nil
}
