package tests_test // or package main_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/algorithms"
)

func setupTestRedis(t *testing.T) (goredis.UniversalClient, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestTokenBucketLimiter_Allow(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	limiter := algorithms.NewTokenBucketLimiter(client)
	ctx := context.Background()
	key := "test:tb:user1"

	capacity := 5.0
	refillRate := 1.0

	// Test burst capacity
	for i := 0; i < 5; i++ {
		res, err := limiter.Allow(ctx, key, capacity, refillRate, 1)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !res.Allowed {
			t.Errorf("expected request %d to be allowed", i+1)
		}
	}

	res, err := limiter.Allow(ctx, key, capacity, refillRate, 1)
	if err != nil {
		t.Fatalf("unexcpected error on depleted request: %v", err)
	}
	if res.Allowed {
		t.Errorf("expected 6th request to be blocked")
	}
}

func TestLeakyBucketLimiter_Allow(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	limiter := algorithms.NewLeakyBucketLimiter(client)
	ctx := context.Background()
	key := "test:lb:user1"

	capacity := 3.0
	leakRate := 1.0

	// Fill bucket up to capacity
	for i := 0; i < 3; i++ {
		res, err := limiter.Allow(ctx, key, capacity, leakRate, 1)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !res.Allowed {
			t.Errorf("expected request %d to be allowed", i+1)
		}
	}

	res, err := limiter.Allow(ctx, key, capacity, leakRate, 1)
	if err != nil {
		t.Fatalf("unexpected error on overflow request: %v", err)
	}
	if res.Allowed {
		t.Errorf("expected request to be blocked due to leaky bucket overflow")
	}
}
