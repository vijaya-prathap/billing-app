package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"billing-app/backend/internal/auth"
)

const (
	// SessionCookie holds the signed session token issued after Google sign-in.
	SessionCookie = "billing_session"
	userKey       = "auth_user"
)

// Auth rejects requests that do not carry a valid session cookie, then re-checks
// the authorized_users table so deactivating an account locks it out at once
// rather than when its session expires. A nil session manager disables
// authentication entirely (AUTH_DISABLED=true), which keeps local development
// and the API tests working without Google.
func Auth(sessions *auth.SessionManager, users auth.UserStore, log *slog.Logger) gin.HandlerFunc {
	if sessions == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		raw, err := c.Cookie(SessionCookie)
		if err != nil || raw == "" {
			authError(c, http.StatusUnauthorized, "unauthorized", "sign in required")
			return
		}
		id, err := sessions.Parse(raw, time.Now())
		if err != nil {
			authError(c, http.StatusUnauthorized, "unauthorized", "session is invalid or has expired")
			return
		}
		ok, err := auth.StillAuthorized(c.Request.Context(), users, id)
		if err != nil {
			log.ErrorContext(c.Request.Context(), "authorization re-check failed",
				slog.String("request_id", c.GetString(RequestIDKey)), slog.Any("error", err))
			authError(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
			return
		}
		if !ok {
			authError(c, http.StatusForbidden, "forbidden", "this Google account is no longer authorized to use the billing app")
			return
		}
		c.Set(userKey, id)
		c.Next()
	}
}

// CurrentUser returns the identity stored by Auth for this request.
func CurrentUser(c *gin.Context) (auth.Identity, bool) {
	v, ok := c.Get(userKey)
	if !ok {
		return auth.Identity{}, false
	}
	id, ok := v.(auth.Identity)
	return id, ok
}

func authError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{
		"code":       code,
		"message":    message,
		"request_id": c.GetString(RequestIDKey),
	}})
}
