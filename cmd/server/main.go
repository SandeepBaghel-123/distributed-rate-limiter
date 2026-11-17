package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/algorithms"
	custommw "github.com/sandeepbaghel/rate-limiter-engine/internal/middleware"
	rds "github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

func main() {
	// 1. Initialize Redis Client
	redisClient, err := rds.NewRedisClient("localhost:6379")
	if err != nil {
		log.Fatalf("Initialization error: %v", err)
	}
	log.Println("Successfully connected to Redis container.")

	// 2. Initialize Limiters for both Algorithms
	tokenBucketLimiter := algorithms.NewTokenBucketLimiter(redisClient.UniversalClient)
	leakyBucketLimiter := algorithms.NewLeakyBucketLimiter(redisClient.UniversalClient)
	fixedWindowLimiter := algorithms.NewFixedWindowLimiter(redisClient.UniversalClient)
	slidingWindowLogLimiter := algorithms.NewSlidingWindowLogLimiter(redisClient.UniversalClient)
	slidingWindowCounterLimiter := algorithms.NewSlidingWindowCounterLimiter(redisClient.UniversalClient)

	// 3. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// 4. Route Group 1: Token Bucket Protected Endpoint
	r.Group(func(r chi.Router) {
		r.Use(custommw.TokenBucketRateLimitMiddleware(tokenBucketLimiter))
		r.Get("/v1/token-resource", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Token bucket request allowed."}`))
		})
	})

	// 5. Route Group 2: Leaky Bucket Protected Endpoint (Traffic Shaping)
	r.Group(func(r chi.Router) {
		r.Use(custommw.LeakyBucketRatetLimitMiddleware(leakyBucketLimiter))
		r.Get("/v1/leaky-resource", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Leaky bucket request processed."}`))
		})
	})

	// 6. Route Group 3: Fixed Window Protected Endpoint (Traffic Shaping)
	r.Group(func(r chi.Router) {
		r.Use(custommw.FixedWindowRatetLimitMiddleware(fixedWindowLimiter))
		r.Get("/v1/fixed-resource", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Fixed Window request processed."}`))
		})
	})

	// 7. Route Group 3: Sliding Window Log Protected Endpoint (Traffic Shaping)
	r.Group(func(r chi.Router) {
		r.Use(custommw.SlidingWindowLogRatetLimitMiddleware(slidingWindowLogLimiter))
		r.Get("/v1/sliding-log-resource", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Sliding Window Log request allowed."}`))
		})
	})

	// 8. Route Group 3: Sliding Window Counter Protected Endpoint (Traffic Shaping)
	r.Group(func(r chi.Router) {
		r.Use(custommw.SlidingWindowCounterRatetLimitMiddleware(slidingWindowCounterLimiter))
		r.Get("/v1/sliding-counter-resource", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Sliding Window Counter request allowed."}`))
		})
	})

	// 9. Start Server
	log.Println("Rate limiter server running on port 8080 with Token, Leaky Bucket, Fixed Window, Sliding Window Log, and Sliding Window Counter algorithms...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
