package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	googleIssuer   = "https://accounts.google.com"
	googleJWKSURL  = "https://www.googleapis.com/oauth2/v3/certs"
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

// GoogleOAuth runs the authorization code flow with PKCE against Google and
// verifies the ID token in the token response against Google's published keys.
// Keys are fetched lazily and cached, so construction needs no network access.
type GoogleOAuth struct {
	cfg      *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewGoogleOAuth(ctx context.Context, clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
			Endpoint:     oauth2.Endpoint{AuthURL: googleAuthURL, TokenURL: googleTokenURL},
		},
		verifier: oidc.NewVerifier(googleIssuer, oidc.NewRemoteKeySet(ctx, googleJWKSURL), &oidc.Config{ClientID: clientID}),
	}
}

func (g *GoogleOAuth) AuthCodeURL(state, codeVerifier string) string {
	return g.cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(codeVerifier))
}

func (g *GoogleOAuth) Exchange(ctx context.Context, code, codeVerifier string) (GoogleAccount, error) {
	token, err := g.cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return GoogleAccount{}, fmt.Errorf("%w: code exchange failed: %v", ErrInvalidToken, err)
	}
	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		return GoogleAccount{}, fmt.Errorf("%w: token response has no id_token", ErrInvalidToken)
	}

	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return GoogleAccount{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return GoogleAccount{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.Email == "" || !claims.EmailVerified {
		return GoogleAccount{}, fmt.Errorf("%w: email address not verified by Google", ErrInvalidToken)
	}
	return GoogleAccount{
		Subject: idToken.Subject,
		Email:   strings.ToLower(strings.TrimSpace(claims.Email)),
		Name:    strings.TrimSpace(claims.Name),
		Picture: claims.Picture,
	}, nil
}
