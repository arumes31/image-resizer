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

// CSRFHeader must accompany UI uploads from clients that do not send
// Sec-Fetch-Site. Browsers only attach custom headers to cross-origin requests
// after a successful CORS preflight, which the UI routes never grant, so a
// forged form or fetch from another site cannot supply it.
const CSRFHeader = "X-Requested-With"

// SameOrigin is the CSRF defence for the web UI, which may carry ambient
// credentials (cached Basic auth or a session from an authenticating proxy).
// Requests that browsers mark as cross-site via Sec-Fetch-Site are rejected.
// Requests without Sec-Fetch-Site must carry CSRFHeader instead. Neither check
// compares Origin with Host, so both keep working behind reverse proxies that
// rewrite the Host header.
func SameOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.GetHeader("Sec-Fetch-Site") {
		case "same-origin", "none":
			c.Next()
		case "":
			if c.GetHeader(CSRFHeader) == "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Missing " + CSRFHeader + " header"})
				c.Abort()
				return
			}
			c.Next()
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "Cross-origin request rejected"})
			c.Abort()
		}
	}
}
