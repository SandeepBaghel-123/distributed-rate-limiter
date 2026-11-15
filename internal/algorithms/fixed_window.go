package algorithms

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

type FixedWindowLimiter struct {
	client goredis.UniversalClient
	script *goredis.Script
}

// NewFixedWindowLimiter initializes the Fixed Window limiter with the lua script.
func NewFixedWindowLimiter(client goredis.UniversalClient) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		client: client,
		script: goredis.NewScript(redis.FixedWindowScript),
	}
}

// Allow evaluates whether a request can pass through the fixed window filter.
func (fw *FixedWindowLimiter) Allow(ctx context.Context, key string, windowSize float64, limit float64, requested int) (*RateLimiterResult, error) {
	now := float64(time.Now().Unix())

	res, err := fw.script.Run(ctx, fw.client, []string{key}, windowSize, limit, requested, now).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to execute fixed window lua script: %w", err)
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
