package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/sandeepbaghel/rate-limiter-engine/internal/algorithms"
)

// rateLimitEvaluator defines a common signature for any underlying algorithm's Allow method
type rateLimitEvaluator func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error)

// baseRateLimitMiddleware encapsulates the shared boilerplate logic for all rate-limitint algorithms.
func baseRateLimitMiddleware(algorithmName string, errorMsg string, quota int, windowSeconds int, evaluate rateLimitEvaluator) func(http.Handler) http.Handler {
	policy := fmt.Sprintf(`"%s";q=%d;w=%d`, algorithmName, quota, windowSeconds)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. Extract identifier (Remote IP)
			identifier := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				identifier = host
			}

			// Construct dynamic redis key based on algorithm and identifier
			limitKey := "rl:" + algorithmName + ":{" + identifier + "}:resource"

			// 2. Evaluate rate limit using the passed strategy
			result, err := evaluate(ctx, limitKey)
			if err != nil {
				// Fail-open posture: allow request if Redis errors out
				next.ServeHTTP(w, r)
				return
			}

			// 4. Set standard IETF Rate Limit headers
			w.Header().Set("RateLimit-Policy", policy)
			w.Header().Set("RateLimit", `"`+algorithmName+`";r=`+strconv.FormatInt(result.Remaining, 10))

			// 5. Handle quota rejection
			if !result.Allowed {
				retryAfterSec := (result.RetryAfterMs + 999) / 1000
				if retryAfterSec < 1 {
					retryAfterSec = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSec, 10))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "` + errorMsg + `"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TokenBucketRateLimitMiddleware wraps the Token Bucket limiter.
func TokenBucketRateLimitMiddleware(limiter *algorithms.TokenBucketLimiter) func(http.Handler) http.Handler {
	const (
		capacity   = 10
		refillRate = 2
	)

	return baseRateLimitMiddleware("tb", "Rate limit exceeded. Token bucket depleted", capacity, refillRate, func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Capacity: 10, Refill rate: 2.0, Requested: 1
		return limiter.Allow(ctx, key, capacity, refillRate, 1)
	})
}

// LeakyBucketRatetLimitMiddleware wraps the Leaky Bucket limiter.
func LeakyBucketRatetLimitMiddleware(limiter *algorithms.LeakyBucketLimiter) func(http.Handler) http.Handler {
	const (
		capacity = 10
		leakRate = 2
	)

	return baseRateLimitMiddleware("lb", "Rate limit exceeded. Leaky bucket overflowed", capacity, leakRate, func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Capacity: 10, Refill rate: 2.0, Requested: 1
		return limiter.Allow(ctx, key, capacity, leakRate, 1)
	})
}

// FixedWindowRatetLimitMiddleware wraps the Fixed Window limiter.
func FixedWindowRatetLimitMiddleware(limiter *algorithms.FixedWindowLimiter) func(http.Handler) http.Handler {
	const (
		windowSeconds = 60
		quota         = 5
	)

	return baseRateLimitMiddleware("fw", "Rate limit exceeded. Fixed Window limit reached.", quota, windowSeconds, func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Window size: 60 seconds, Limit: 5 requests, Requested: 1
		return limiter.Allow(ctx, key, windowSeconds, quota, 1)
	})
}

// SlidingWindowLogRatetLimitMiddleware wraps the Sliding Window Log limiter.
func SlidingWindowLogRatetLimitMiddleware(limiter *algorithms.SlidingWindowLogLimiter) func(http.Handler) http.Handler {
	const (
		windowSeconds = 60
		quota         = 5
	)

	return baseRateLimitMiddleware("swl", "Rate limit exceeded. Sliding Window Log limit reached.", quota, windowSeconds, func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Window size: 60 seconds, Limit: 5 requests, Requested: 1
		return limiter.Allow(ctx, key, windowSeconds, quota, 1)
	})
}

// SlidingWindowCounterRatetLimitMiddleware wraps the Sliding Window Counter limiter.
func SlidingWindowCounterRatetLimitMiddleware(limiter *algorithms.SlidingWindowCounterLimiter) func(http.Handler) http.Handler {
	const (
		windowSeconds = 60
		quota         = 5
	)

	return baseRateLimitMiddleware("swc", "Rate limit exceeded. Sliding Window Counter limit reached.", quota, windowSeconds, func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Window size: 60 seconds, Limit: 5 requests, Requested: 1
		return limiter.Allow(ctx, key, windowSeconds, quota, 1)
	})
}
