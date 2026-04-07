// services/gateway/internal/ratelimit/ratelimit_test.go
package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/ratelimit"
)

func TestTokenBucket_AllowsWithinLimit(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     10,
		Capacity: 10,
	})

	for i := 0; i < 10; i++ {
		if !limiter.Allow("user-1") {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}
}

func TestTokenBucket_BlocksOverLimit(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     2,
		Capacity: 2,
	})

	limiter.Allow("user-1")
	limiter.Allow("user-1")

	if limiter.Allow("user-1") {
		t.Fatal("expected third request to be blocked")
	}
}

func TestTokenBucket_SeparateKeys(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     1,
		Capacity: 1,
	})

	if !limiter.Allow("user-1") {
		t.Fatal("expected user-1 first request to be allowed")
	}
	if !limiter.Allow("user-2") {
		t.Fatal("expected user-2 first request to be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     1,
		Capacity: 1,
	})

	handler := ratelimit.Middleware(ratelimit.MiddlewareParams{
		Limiter: limiter,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-User-ID")
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-User-ID", "42")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-User-ID", "42")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
}
