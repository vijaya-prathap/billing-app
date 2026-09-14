package middleware

import "github.com/gin-gonic/gin"

// Auth is a deliberate passthrough until an identity provider is chosen; the API
// routes are already mounted behind it, so real checks need no router changes.
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
