package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/auth"
	"billing-app/backend/internal/middleware"
)

const (
	// StateCookie carries the pending login (state + PKCE verifier) between the
	// redirect to Google and the callback. Scoped to the auth routes only.
	StateCookie     = "billing_oauth_state"
	stateCookiePath = "/api/v1/auth/google"

	// The SPA lives at the site root; the callback sends the browser back there
	// with an auth_error query parameter when sign-in did not produce a session.
	appHome            = "/"
	authErrorParam     = "auth_error"
	authErrorDenied    = "denied"    // Google account is not an authorized user
	authErrorFailed    = "failed"    // state mismatch, exchange or verification failure
	authErrorCancelled = "cancelled" // the person backed out at Google
)

type AuthHandler struct {
	svc *auth.Service // nil when authentication is disabled
	log *slog.Logger
}

func NewAuthHandler(svc *auth.Service, log *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

type userResponse struct {
	User auth.Identity `json:"user"`
}

// Config tells the frontend whether to show the login screen.
func (h *AuthHandler) Config(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": h.svc != nil})
}

// GoogleLogin starts the OAuth 2.0 authorization code flow: it remembers a
// random state and PKCE verifier in a short-lived signed cookie, then sends the
// browser to Google. Nothing about the person is known yet.
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	pending, cookie, err := h.svc.States.Issue(time.Now())
	if err != nil {
		handleServiceError(c, h.log, err)
		return
	}
	h.setCookie(c, StateCookie, cookie, stateCookiePath, int(h.svc.States.TTL().Seconds()))
	c.Redirect(http.StatusFound, h.svc.OAuth.AuthCodeURL(pending.State, pending.CodeVerifier))
}

// GoogleCallback is where Google redirects after authenticating the person.
// It verifies the state, redeems the code for a verified Google account, checks
// that account against authorized_users, and only then issues a session cookie.
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString(middleware.RequestIDKey)

	stateCookie, _ := c.Cookie(StateCookie)
	h.setCookie(c, StateCookie, "", stateCookiePath, -1) // single use

	if reason := c.Query("error"); reason != "" {
		h.log.InfoContext(ctx, "google sign-in cancelled", slog.String("request_id", requestID), slog.String("reason", reason))
		h.redirectWithError(c, authErrorCancelled)
		return
	}

	pending, err := h.svc.States.Verify(stateCookie, c.Query("state"), time.Now())
	if err != nil {
		h.log.WarnContext(ctx, "oauth state rejected", slog.String("request_id", requestID), slog.Any("error", err))
		h.redirectWithError(c, authErrorFailed)
		return
	}

	code := c.Query("code")
	if code == "" {
		h.log.WarnContext(ctx, "oauth callback without code", slog.String("request_id", requestID))
		h.redirectWithError(c, authErrorFailed)
		return
	}

	account, err := h.svc.OAuth.Exchange(ctx, code, pending.CodeVerifier)
	if err != nil {
		h.log.WarnContext(ctx, "google credential rejected", slog.String("request_id", requestID), slog.Any("error", err))
		h.redirectWithError(c, authErrorFailed)
		return
	}

	id, err := h.svc.Authorize(ctx, account)
	if err != nil {
		if errors.Is(err, auth.ErrNotAllowed) {
			h.log.WarnContext(ctx, "sign-in refused: google account is not an authorized user",
				slog.String("request_id", requestID), slog.String("email", account.Email), slog.String("google_sub", account.Subject))
			h.redirectWithError(c, authErrorDenied)
			return
		}
		h.log.ErrorContext(ctx, "authorization lookup failed", slog.String("request_id", requestID), slog.Any("error", err))
		h.redirectWithError(c, authErrorFailed)
		return
	}

	token, expires := h.svc.Sessions.Issue(id, time.Now())
	h.setCookie(c, middleware.SessionCookie, token, "/", int(time.Until(expires).Seconds()))
	h.log.InfoContext(ctx, "user signed in",
		slog.String("request_id", requestID), slog.String("email", id.Email), slog.Int64("user_id", id.UserID))
	c.Redirect(http.StatusFound, appHome)
}

// Me returns the signed-in user; the Auth middleware has already validated the cookie.
func (h *AuthHandler) Me(c *gin.Context) {
	id, ok := middleware.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "sign in required", nil)
		return
	}
	c.JSON(http.StatusOK, userResponse{User: id})
}

// Logout clears the cookie. Sessions are stateless, so the token simply stops being sent.
func (h *AuthHandler) Logout(c *gin.Context) {
	h.setCookie(c, middleware.SessionCookie, "", "/", -1)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) redirectWithError(c *gin.Context, reason string) {
	c.Redirect(http.StatusFound, appHome+"?"+url.Values{authErrorParam: {reason}}.Encode())
}

// setCookie applies the hardening every auth cookie gets: HttpOnly so scripts
// cannot read it, SameSite=Lax so it is not sent on cross-site POSTs yet still
// survives Google's top-level redirect back, and Secure whenever configured.
func (h *AuthHandler) setCookie(c *gin.Context, name, value, path string, maxAge int) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.svc.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if maxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	} else {
		cookie.Expires = time.Unix(0, 0)
	}
	http.SetCookie(c.Writer, cookie)
}
