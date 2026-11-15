package algorithms

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

type SlidingWindowLogLimiter struct {
	client goredis.UniversalClient
	script *goredis.Script
}

// NewSlidingWindowLogLimiter initializes the Sliding Window Log limiter.
func NewSlidingWindowLogLimiter(client goredis.UniversalClient) *SlidingWindowLogLimiter {
	return &SlidingWindowLogLimiter{
		client: client,
		script: goredis.NewScript(redis.SlidingWindowLogScript),
	}
}

// Allow evaluates whether a request can pass through the sliding window log filter.
func (sw *SlidingWindowLogLimiter) Allow(ctx context.Context, key string, windowSize float64, limit float64, requested int) (*RateLimiterResult, error) {
	winSec := int(windowSize)
	limInt := int(limit)

	// Generate a guaranteed unique member ID even in tight loops
	memberId := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), requested, rand.Int())

	res, err := sw.script.Run(ctx, sw.client, []string{key}, winSec, limInt, requested, memberId).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to execute sliding window log lua script: %w", err)
	}

	resultSlice, ok := res.([]interface{})
	if !ok || len(resultSlice) < 3 {
		return nil, fmt.Errorf("unexpected lua script response format")
	}

	allowed := toInt64(resultSlice[0]) == 1
	remaining := toInt64(resultSlice[1])
	retryAfterSec := toInt64(resultSlice[2])

	result := &RateLimiterResult{
		Allowed:      allowed,
		Remaining:    remaining,
		RetryAfterMs: retryAfterSec * 1000,
	}

	return result, nil
}
