package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// UIBasicAuth protects the web UI with HTTP Basic authentication when a
// password is configured. With an empty password it is a no-op, which keeps
// existing deployments (e.g. behind an authenticating reverse proxy) working.
func UIBasicAuth(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if password == "" {
			c.Next()
			return
		}

		user, pass, ok := c.Request.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1
		if !ok || !userOK || !passOK {
			c.Header("WWW-Authenticate", `Basic realm="Image Resizer", charset="UTF-8"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

// SameOrigin rejects state-changing requests that browsers mark as
// cross-site via the Sec-Fetch-Site header, a lightweight CSRF defence for
// the web UI. It does not compare Origin with Host, so it keeps working behind
// reverse proxies that rewrite the Host header. Requests without the header
// (non-browser clients, very old browsers) are allowed.
func SameOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.GetHeader("Sec-Fetch-Site") {
		case "", "same-origin", "none":
			c.Next()
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "Cross-origin request rejected"})
			c.Abort()
		}
	}
}
