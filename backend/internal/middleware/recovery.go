package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// net/http uses this sentinel to abort a response silently; re-panic to keep that contract.
			if rec == http.ErrAbortHandler {
				panic(rec)
			}

			requestID := c.GetString(RequestIDKey)
			log.ErrorContext(c.Request.Context(), "panic recovered",
				slog.String("request_id", requestID),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)

			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":       "internal_error",
					"message":    "an unexpected error occurred",
					"request_id": requestID,
				},
			})
		}()

		c.Next()
	}
}
