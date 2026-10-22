package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	rl := NewRateLimiter(2)
	rl.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if !rl.Allow("a") {
			t.Fatalf("burst request %d should be allowed", i+1)
		}
	}
	if rl.Allow("a") {
		t.Fatal("third request should be limited")
	}
	if !rl.Allow("b") {
		t.Fatal("other clients must not be affected")
	}
	now = now.Add(30 * time.Second)
	if !rl.Allow("a") {
		t.Fatal("token should refill after 30s")
	}
}

func newTestRouter(h ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Any("/", append(h, func(c *gin.Context) { c.Status(http.StatusOK) })...)
	return r
}

func TestUIBasicAuth(t *testing.T) {
	r := newTestRouter(UIBasicAuth("admin", "secret"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got %d", w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.SetBasicAuth("admin", "secret")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with credentials, got %d", w.Code)
	}

	open := newTestRouter(UIBasicAuth("admin", ""))
	w = httptest.NewRecorder()
	open.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if w.Code != http.StatusOK {
		t.Fatalf("expected open access without a password, got %d", w.Code)
	}
}

func TestSameOrigin(t *testing.T) {
	r := newTestRouter(SameOrigin())
	for site, want := range map[string]int{
		"":            http.StatusOK,
		"same-origin": http.StatusOK,
		"none":        http.StatusOK,
		"cross-site":  http.StatusForbidden,
		"same-site":   http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		if site != "" {
			req.Header.Set("Sec-Fetch-Site", site)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Errorf("Sec-Fetch-Site %q: got %d, want %d", site, w.Code, want)
		}
	}
}
