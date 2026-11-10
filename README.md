# distributed-rate-limiter
A production-grade distributed rate-limiter engine in Go implementing Token and Leaky Bucket algorithms via atomic Redis Lua scripts.


## 📊 Performance & Benchmarks

The rate limiter engine was load-tested locally using `hey` under high-concurrency traffic spikes to evaluate throughput, latency distribution, and algorithm correctness under saturation.

### Test Configuration
- **Tool:** `hey`
- **Concurrency (`-c`):** 100 parallel workers
- **Duration (`-z`):** 30 seconds
- **Target Endpoint:** `/v1/leaky-resource` (Backed by Redis atomic Lua scripts)

### Results Summary
- **Total Requests Handled:** ~343,400
- **Throughput:** **11,445.16 Requests/Sec**
- **Status Code Distribution:**
  - `200 OK` (Allowed): 6,900 responses
  - `429 Too Many Requests` (Throttled): 336,514 responses

### Latency Percentiles
- **Fastest:** 0.5 ms
- **p50 (Median):** 8.5 ms
- **p90:** 11.6 ms
- **p95:** 13.1 ms
- **p99:** 16.5 ms
- **Slowest:** 109.1 ms

### Key Takeaway
The engine successfully protects downstream services by aggressively throttling excess traffic while maintaining a stable p99 latency under 17ms, proving the efficiency of atomic Redis Lua state evaluations under high concurrent loads.


### 2. Token Bucket Algorithm Benchmark (`/v1/token-resource`)
- **Throughput:** **11,254.06 Requests/Sec**
- **Status Code Distribution:**
  - `200 OK` (Allowed - Burst accommodated): 34,137 responses
  - `429 Too Many Requests` (Throttled): 303,563 responses
- **Latency Percentiles:**
  - **p50:** 8.6 ms
  - **p90:** 11.7 ms
  - **p99:** 16.7 ms
