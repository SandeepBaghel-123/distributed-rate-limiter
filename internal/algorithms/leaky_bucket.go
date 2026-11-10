package algorithms

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

type LeakyBucketLimiter struct {
	client goredis.UniversalClient
	script *goredis.Script
}

// NewLeakyBucketLimiter initializes the Leaky Bucket limiter with the Lua script.
func NewLeakyBucketLimiter(client goredis.UniversalClient) *LeakyBucketLimiter {
	return &LeakyBucketLimiter{
		client: client,
		script: goredis.NewScript(redis.LeakyBucketScript),
	}
}

func (lb *LeakyBucketLimiter) Allow(ctx context.Context, key string, capacity float64, leakRate float64, requested int) (*RateLimiterResult, error) {
	// Use fractional seconds for precise real-time water leakage calculation
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := lb.script.Run(ctx, lb.client, []string{key}, now, capacity, leakRate, requested).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to exceute leaky bucket lua script: %w", err)
	}

	resultSlice, ok := res.([]interface{})
	if !ok || len(resultSlice) < 2 {
		return nil, fmt.Errorf("unexpected lua script response format")
	}

	allowed := toInt64(resultSlice[0]) == 1
	currentWater := toInt64(resultSlice[1]) // Sorted as current water level/count

	result := &RateLimiterResult{
		Allowed:      allowed,
		CurrentCount: currentWater,
	}

	if allowed {
		// Remaining capacity approximation based on total capacity minue current water
		remaining := int64(capacity) - currentWater
		if remaining < 0 {
			remaining = 0
		}
		result.Remaining = remaining
	} else if len(resultSlice) >= 3 {
		// If rejected, set retry after duration in milliseconds
		result.RetryAfterMs = toInt64(resultSlice[2]) * 1000
	}

	return result, nil
}
