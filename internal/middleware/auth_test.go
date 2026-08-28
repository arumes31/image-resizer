package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name       string
		configured string
		provided   string
		wantStatus int
	}{
		{name: "valid", configured: "long-random-key", provided: "long-random-key", wantStatus: http.StatusNoContent},
		{name: "missing", configured: "long-random-key", wantStatus: http.StatusUnauthorized},
		{name: "invalid", configured: "long-random-key", provided: "wrong", wantStatus: http.StatusUnauthorized},
		{name: "server key missing", provided: "key", wantStatus: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(APIKeyAuth(test.configured))
			router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if test.provided != "" {
				req.Header.Set("X-API-Key", test.provided)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
