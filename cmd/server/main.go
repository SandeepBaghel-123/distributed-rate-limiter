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

	// 6. Start Server
	log.Println("Rate limiter server running on port 8080 with Token and Leaky Bucket algorithms...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
