package algorithms

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

type SlidingWindowCounterLimiter struct {
	client goredis.UniversalClient
	script goredis.Script
}

// NewSlidingWindowCounterLimiter initializes the Sliding Window Counter limiter.
func NewSlidingWindowCounterLimiter(client goredis.UniversalClient) *SlidingWindowCounterLimiter {
	return &SlidingWindowCounterLimiter{
		client: client,
		script: *goredis.NewScript(redis.SlidingWindowCounterScript),
	}
}

func (swc *SlidingWindowCounterLimiter) Allow(ctx context.Context, key string, windowSize float64, limit float64, requested int) (*RateLimiterResult, error) {
	now := float64(time.Now().Unix())
	winSec := int(windowSize)
	limInt := int(limit)

	res, err := swc.script.Run(ctx, swc.client, []string{key}, winSec, limInt, requested, now).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to execute sliding window counter lua script: %w", err)
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
