package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")

		if key == "" || apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing API Key"})
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing API Key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
