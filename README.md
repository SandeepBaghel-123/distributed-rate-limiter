# Distributed Rate Limiter

A Go HTTP middleware demo that implements five rate-limiting algorithms with Redis. Multiple Go server instances share the same quota state. Each rate-limit decision runs in a Redis Lua script so the quota check and update happen atomically.

The quota key uses the client's IP address and the algorithm. Reconnecting from the same IP or switching between server instances backed by the same Redis instance uses the same quota. Each demo endpoint has its own quota.

## Endpoints

| GET endpoint | Algorithm | Policy |
| --- | --- | --- |
| `/v1/token-resource` | Token Bucket | Starts with up to 10 tokens and refills at 2 tokens per second. Each accepted request spends one token. |
| `/v1/leaky-resource` | Leaky Bucket | Holds up to 10 units and drains at 2 units per second. Accepted requests run immediately. |
| `/v1/fixed-resource` | Fixed Window | Allows 5 requests per aligned 60-second window. The counter resets at the window boundary. |
| `/v1/sliding-log-resource` | Sliding Window Log | Allows 5 requests per rolling 60-second window. Redis stores individual timestamps with one-second resolution. |
| `/v1/sliding-counter-resource` | Sliding Window Counter | Approximates a rolling limit of 5 requests per 60 seconds by weighting the previous window's count. |

A rejected request returns `429 Too Many Requests` with a JSON body and a `Retry-After` value in seconds. If Redis returns an error during a request, the middleware **fails open** and lets the request proceed. The server requires Redis to be reachable at startup.

## Quick start

Install Go 1.27.3 or newer and Docker with Compose. Install `hey` if you want to run the benchmarks.

```bash
git clone https://github.com/SandeepBaghel-123/distributed-rate-limiter.git
cd distributed-rate-limiter
docker compose up -d redis
docker compose exec redis redis-cli ping
go run ./cmd/server
```

Wait for Redis to respond with `PONG` before starting the server. The API listens on port 8080 by default.

```bash
curl -i http://127.0.0.1:8080/v1/fixed-resource
```

| Environment variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8080` |
| `REDIS_ADDR` | `localhost:6379` |

## Demo a shared quota across two servers

Keep the first server running and start a second one in another terminal from the same checkout. Both use the same default Redis address.

```powershell
$env:HTTP_ADDR = ":8081"
go run ./cmd/server
```

On Bash or Zsh, use `HTTP_ADDR=:8081 go run ./cmd/server` instead.

From a third terminal, send three requests to port 8080 and three to port 8081. Start with an unused Fixed Window quota and run all six requests within the same clock minute. If you used the quick-start request above, wait for the next minute before running the demo.

```bash
for port in 8080 8081; do
  for attempt in 1 2 3; do
    curl -s -o /dev/null -w "$port %{http_code}\n" "http://127.0.0.1:$port/v1/fixed-resource"
  done
done
```

```powershell
foreach ($port in 8080, 8081) {
    foreach ($attempt in 1..3) {
        curl.exe -s -o NUL -w "$port %{http_code}\n" "http://127.0.0.1:${port}/v1/fixed-resource"
    }
}
```

Expected output is three `200` responses from port 8080, followed by two `200` responses and one `429` from port 8081. The same client gets five successful requests across both servers.

## Tests

```bash
go test ./...
```

The algorithm tests use miniredis and cover quota exhaustion for all five strategies. The Sliding Window Log test also checks that requests are admitted again after older entries expire.

## Local HTTP throughput under sustained throttling

Each endpoint was measured in a single local `hey` run with 100 concurrent workers for about 30 seconds. Results include both accepted and throttled responses. Almost every response was a `429`, so the end-to-end latency percentiles mainly describe how quickly the server rejected excess traffic.

| Algorithm | Duration (s) | HTTP requests/s | 200 OK | 429 Too Many Requests | p50 (ms) | p95 (ms) | p99 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Token Bucket | 30.0070 | 10,870 | 69 | 326,121 | 8.8 | 13.6 | 17.8 |
| Leaky Bucket | 30.0050 | 11,805 | 69 | 354,153 | 8.2 | 12.5 | 16.0 |
| Fixed Window | 30.0044 | 11,728 | 10 | 351,892 | 8.3 | 12.6 | 15.9 |
| Sliding Window Log | 30.0048 | 11,594 | 5 | 347,884 | 8.3 | 12.7 | 15.8 |
| Sliding Window Counter | 30.0036 | 12,183 | 5 | 365,540 | 8.1 | 11.6 | 14.5 |

The Fixed Window count of 10 is consistent with crossing an aligned reset and consuming five requests on each side. The two bucket algorithms each admitted 69 requests, close to the approximately 70 expected from a fresh bucket under sustained load for 30 seconds.

```bash
hey -z 30s -c 100 http://localhost:8080/v1/token-resource
hey -z 30s -c 100 http://127.0.0.1:8080/v1/leaky-resource
hey -z 30s -c 100 http://127.0.0.1:8080/v1/fixed-resource
hey -z 30s -c 100 http://127.0.0.1:8080/v1/sliding-log-resource
hey -z 30s -c 100 http://127.0.0.1:8080/v1/sliding-counter-resource
```

Start each run with fresh quota state. Wait at least five seconds after traffic stops for either bucket to recover. Allow up to two minutes of idle time to clear prior usage for the window-based endpoints. Run one benchmark at a time.

## Scope

- The demo uses standalone Redis and identifies clients by their directly connected IP. Clients sharing an IP also share an endpoint's quota. Behind a reverse proxy, the proxy may appear as the client.
- The window-based routes advertise their quota in `RateLimit-Policy`. The two bucket routes currently emit a window-style value that does not express their burst and sustained rates. Use the endpoint table for those policies.
- The Compose file also defines Postgres. The rate limiter uses Redis only, so the quick start launches just the Redis service.
