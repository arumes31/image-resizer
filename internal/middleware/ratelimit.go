package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// defaultMaxClients caps how many client buckets are tracked at once.
	defaultMaxClients = 100_000
	// sweepInterval is how often idle buckets are dropped.
	sweepInterval = time.Minute
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-client-IP token bucket limiter.
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	rate       float64 // tokens per second
	burst      float64
	maxClients int
	lastSweep  time.Time
	now        func() time.Time
}

// NewRateLimiter allows perMinute requests per client IP per minute, with a
// burst of the same size.
func NewRateLimiter(perMinute int) *RateLimiter {
	return &RateLimiter{
		buckets:    make(map[string]*bucket),
		rate:       float64(perMinute) / 60.0,
		burst:      float64(perMinute),
		maxClients: defaultMaxClients,
		now:        time.Now,
	}
}

// Allow reports whether a request from key may proceed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	// Sweep on a fixed schedule rather than on every request, so the cost of
	// an admission decision does not grow with the number of tracked clients.
	if now.Sub(rl.lastSweep) >= sweepInterval {
		rl.sweep(now)
		rl.lastSweep = now
	}

	b, ok := rl.buckets[key]
	if !ok {
		// Fail closed for new clients once the table is full, so client churn
		// cannot grow the map without bound. Known clients are unaffected and
		// idle buckets are freed by the next sweep.
		if len(rl.buckets) >= rl.maxClients {
			return false
		}
		b = &bucket{tokens: rl.burst, lastSeen: now}
		rl.buckets[key] = b
	}
	b.tokens += now.Sub(b.lastSeen).Seconds() * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep drops buckets that have been idle long enough to refill completely.
// Such a bucket is indistinguishable from a fresh one, so removing it never
// grants a client more than it would otherwise get.
func (rl *RateLimiter) sweep(now time.Time) {
	refill := time.Duration(rl.burst / rl.rate * float64(time.Second))
	for k, b := range rl.buckets {
		if now.Sub(b.lastSeen) >= refill {
			delete(rl.buckets, k)
		}
	}
}

// Middleware rejects requests over the limit with 429 Too Many Requests.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests, please slow down"})
			c.Abort()
			return
		}
		c.Next()
	}
}
