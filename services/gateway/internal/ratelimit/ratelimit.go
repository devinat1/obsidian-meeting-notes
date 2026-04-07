// services/gateway/internal/ratelimit/ratelimit.go
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

type bucket struct {
	tokens   float64
	lastTime time.Time
}

type TokenBucket struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	capacity float64
}

type TokenBucketParams struct {
	Rate     float64
	Capacity float64
}

func NewTokenBucket(params TokenBucketParams) *TokenBucket {
	return &TokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     params.Rate,
		capacity: params.Capacity,
	}
}

func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()

	b, exists := tb.buckets[key]
	if !exists {
		tb.buckets[key] = &bucket{
			tokens:   tb.capacity - 1,
			lastTime: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > tb.capacity {
		b.tokens = tb.capacity
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

type MiddlewareParams struct {
	Limiter *TokenBucket
	KeyFunc func(r *http.Request) string
}

func Middleware(params MiddlewareParams) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := params.KeyFunc(r)
			if !params.Limiter.Allow(key) {
				httputil.RespondError(w, http.StatusTooManyRequests, "Rate limit exceeded. Try again later.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
