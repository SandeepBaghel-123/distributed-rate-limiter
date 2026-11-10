package middleware

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sandeepbaghel/rate-limiter-engine/internal/algorithms"
)

// rateLimitEvaluator defines a common signature for any underlying algorithm's Allow method
type rateLimitEvaluator func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error)

// baseRateLimitMiddleware encapsulates the shared boilerplate logic for all rate-limitint algorithms.
func baseRateLimitMiddleware(algorithmName string, errorMsg string, evaluate rateLimitEvaluator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. Extract indentifier (API Key or fallback to Remote IP)
			indentifier := r.Header.Get("X-API-Key")
			if indentifier == "" {
				indentifier = r.RemoteAddr
			}

			// Construct dynamic redis key based on algorithm and identifier
			limitKey := "rl:" + algorithmName + ":{" + indentifier + "}:resource"

			// 2. Evaluate rate limit using the passed strategy
			result, err := evaluate(ctx, limitKey)
			if err != nil {
				// Fail-open posture: allow request if Redis errors out
				next.ServeHTTP(w, r)
				return
			}

			// 4. Set standard IETF Rate Limit headers
			w.Header().Set("RateLimit-Policy", `"`+algorithmName+`";q=10;w=5`)
			w.Header().Set("RateLimit", `"`+algorithmName+`";r=`+strconv.FormatInt(result.Remaining, 10))

			// 5. Handle quota rejection
			if !result.Allowed {
				retryAfterSec := (result.RetryAfterMs / 1000) + 1
				if retryAfterSec < 1 {
					retryAfterSec = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSec, 10))
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
	return baseRateLimitMiddleware("tb", "Rate limit exceeded. Token bucket depleted", func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Capacity: 10, Refill rate: 2.0, Requested: 1
		return limiter.Allow(ctx, key, 10.0, 2.0, 1)
	})
}

// LeakyBucketRatetLimitMiddleware wraps the Leaky Bucket limiter.
func LeakyBucketRatetLimitMiddleware(limiter *algorithms.LeakyBucketLimiter) func(http.Handler) http.Handler {
	return baseRateLimitMiddleware("lb", "Rate limit exceeded. Leaky bucket overflowed", func(ctx context.Context, key string) (*algorithms.RateLimiterResult, error) {
		// Capacity: 10, Refill rate: 2.0, Requested: 1
		return limiter.Allow(ctx, key, 10.0, 2.0, 1)
	})
}
