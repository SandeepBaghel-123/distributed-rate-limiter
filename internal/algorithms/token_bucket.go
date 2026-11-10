package algorithms

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

// RateLimiterResult holds the outcome of a rate limit check
type RateLimiterResult struct {
	Allowed			bool
	CurrentCount	int64
	Remaining		int64
	RetryAfterMs	int64
}

type TokenBucketLimiter struct {
	client goredis.UniversalClient
	script *goredis.Script
}

func NewTokenBucketLimiter(client goredis.UniversalClient) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		client: client,
		script: goredis.NewScript(redis.TokenBucketScript),
	}
}

func (tb *TokenBucketLimiter) Allow(ctx context.Context, key string, capacity float64, refillRate float64, requestedTokens int) (*RateLimiterResult, error) {
	// Use fractional seconds for precise real-time calculation
	now := float64(time.Now().UnixNano()) / 1e9
	res, err := tb.script.Run(ctx, tb.client, []string{key}, now, capacity, refillRate, requestedTokens).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to execute token bucket lua script: %w", err)
	}

	resultSlice, ok := res.([]interface{})
	if !ok || len(resultSlice) < 2 {
		return nil, fmt.Errorf("unexpected lua script response format")
	}

	allowed := toInt64(resultSlice[0]) == 1
	currentTokens := toInt64(resultSlice[1])

	result := &RateLimiterResult{
		Allowed:		allowed,
		CurrentCount:	currentTokens,
	}

	if allowed {
		result.Remaining = currentTokens
	} else if len(resultSlice) >= 3 {
		result.RetryAfterMs = toInt64(resultSlice[2]) * 1000
	}

	return result, nil
}

func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int64: return v
	case int: return int64(v)
	case int32: return int64(v)
	case float64: return int64(v)
	case string:
		var i int64
		fmt.Sscanf(v, "%d", &i)
		return i
	default:
		return 0
	}
}