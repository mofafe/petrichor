package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const userIDKey = "auth.user_id"

type TokenVerifier interface {
	UserIDFromToken(token string) (string, error)
}

func RequireAuth(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := verifier.UserIDFromToken(strings.TrimSpace(token))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(userIDKey, userID)
		c.Next()
	}
}

func UserID(c *gin.Context) (string, bool) {
	value, ok := c.Get(userIDKey)
	if !ok {
		return "", false
	}

	userID, ok := value.(string)
	return userID, ok && userID != ""
}
