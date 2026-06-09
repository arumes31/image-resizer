package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("api_key")
		}

		if key != apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing API Key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
