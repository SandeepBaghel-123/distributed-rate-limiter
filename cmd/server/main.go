package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sandeepbaghel/rate-limiter-engine/internal/algorithms"
	custommw "github.com/sandeepbaghel/rate-limiter-engine/internal/middleware"
	rds "github.com/sandeepbaghel/rate-limiter-engine/internal/redis"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")
	httpAddr := envOrDefault("HTTP_ADDR", ":8080")

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	redisClient, err := rds.NewRedisClient(redisAddr)
	if err != nil {
		return fmt.Errorf("connect to Redis at %s: %w", redisAddr, err)
	}
	defer func() {
		if err := redisClient.UniversalClient.Close(); err != nil {
			log.Printf("close Redis client: %v", err)
		}
	}()

	tokenBucketLimiter := algorithms.NewTokenBucketLimiter(redisClient.UniversalClient)
	leakyBucketLimiter := algorithms.NewLeakyBucketLimiter(redisClient.UniversalClient)
	fixedWindowLimiter := algorithms.NewFixedWindowLimiter(redisClient.UniversalClient)
	slidingWindowLogLimiter := algorithms.NewSlidingWindowLogLimiter(redisClient.UniversalClient)
	slidingWindowCounterLimiter := algorithms.NewSlidingWindowCounterLimiter(redisClient.UniversalClient)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Admit requests while the bucket has enough tokens.
	router.With(custommw.TokenBucketRateLimitMiddleware(tokenBucketLimiter)).
		Get("/v1/token-resource", jsonOK(
			`{"status": "success", "message": "Token bucket request allowed."}`,
		))

	// Admit requests when the bucket has room. Accepted requests run immediately.
	router.With(custommw.LeakyBucketRatetLimitMiddleware(leakyBucketLimiter)).
		Get("/v1/leaky-resource", jsonOK(
			`{"status": "success", "message": "Leaky bucket request processed."}`,
		))

	// Count requests within the current fixed time window.
	router.With(custommw.FixedWindowRatetLimitMiddleware(fixedWindowLimiter)).
		Get("/v1/fixed-resource", jsonOK(
			`{"status": "success", "message": "Fixed Window request processed."}`,
		))

	// Count recorded request timestamps within the rolling time window.
	router.With(custommw.SlidingWindowLogRatetLimitMiddleware(slidingWindowLogLimiter)).
		Get("/v1/sliding-log-resource", jsonOK(
			`{"status": "success", "message": "Sliding Window Log request allowed."}`,
		))

	// Estimate rolling usage from the current and previous window counts.
	router.With(custommw.SlidingWindowCounterRatetLimitMiddleware(slidingWindowCounterLimiter)).
		Get("/v1/sliding-counter-resource", jsonOK(
			`{"status": "success", "message": "Sliding Window Counter request allowed."}`,
		))

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting HTTP server on %s with Redis at %s", httpAddr, redisAddr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP on %s: %w", httpAddr, err)
		}
	case <-signalCtx.Done():
		stop()
		log.Print("Shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			if closeErr := server.Close(); closeErr != nil {
				log.Printf("force close HTTP server: %v", closeErr)
			}
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
	}

	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func jsonOK(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}
