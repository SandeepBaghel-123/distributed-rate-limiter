# ⚡ Distributed Rate Limiter Engine

A high-performance, distributed rate-limiter engine implemented in **Go** utilizing **Redis** and **atomic Lua scripts**. This engine provides thread-safe, low-latency traffic shaping and rate limiting across distributed microservices.

It supports multiple core algorithms:
- **Token Bucket Algorithm** (Supports bursty client traffic)
- **Leaky Bucket Algorithm** (Enforces smooth, constant egress rate processing)

---

## 📌 Architecture Overview
```
                    +----------------------+
                    |   HTTP Request /     |
                    |   Client Traffic     |
                    +----------+-----------+
                               |
                               v
                    +----------+-----------+
                    |   Go Rate Limiter    |
                    |       Engine         |
                    +----------+-----------+
                               |
                    (Executes Lua Script)
                               |
                               v
                    +----------+-----------+
                    |      Redis Data      |
                    |     Store (State)    |
                    +----------------------+

```

### Key Engineering Highlights
* **Atomic State Updates:** State checks and updates are executed within Redis using **Lua scripts** to guarantee strict atomicity without relying on heavy application-level distributed locks.
* **Concurrency-Safe:** Designed to handle heavy concurrent request spikes with near-zero lock contention.
* **Low Latency Overhead:** Sub-10ms median latency ($p_{50}$) for client request evaluation.
* **Modular Design:** Simple abstraction interface allowing custom rate-limiting algorithms to be plugged into standard HTTP middleware seamlessly.

---

## 🛠️ Tech Stack & Prerequisites

* **Language:** Go 1.22+
* **State Store:** Redis 7+
* **Deployment:** Docker & Docker Compose
* **Benchmarking Tools:** `hey`, `k6`

---

## 🚀 Quick Start & Local Setup

### 1. Clone the Repository
```bash
git clone [https://github.com/SandeepBaghel-123/distributed-rate-limiter.git](https://github.com/SandeepBaghel-123/distributed-rate-limiter.git)
cd distributed-rate-limiter

```

### 2. Run via Docker Compose

Spin up both the Redis service and the Go HTTP server:

```bash
docker compose up -d --build

```

The service will start listening on `http://localhost:8080`.

---

## 🔌 API Endpoints

| Endpoint | Algorithm | Behavior |
| --- | --- | --- |
| `GET /v1/token-resource` | **Token Bucket** | Accommodates traffic bursts up to max capacity; throttles when tokens deplete. |
| `GET /v1/leaky-resource` | **Leaky Bucket** | Processes requests at a constant leak rate; drops excess requests instantly when full. |

---

## 📊 Performance & Load Testing Benchmarks

The engine was benchmarked under heavy saturation traffic to evaluate throughput, concurrency safety, and latency profiles under load.

### Test Environment

* **Tool:** `hey` CLI
* **Concurrency Level (`-c`):** 100 concurrent connections
* **Test Duration (`-z`):** 30 seconds
* **Environment:** Go 1.22 + Redis 7 running via Docker containers

---

### 1. Token Bucket Algorithm (`/v1/token-resource`)

I tested the Token Bucket algorithm under burst conditions to verify capacity-based allowance and clean throttling upon token exhaustion.

* **Total Requests Processed:** ~337,700
* **Throughput:** **11,254.06 Requests/Sec**
* **Status Code Distribution:**
* `200 OK` (Allowed Bursts): **34,137**
* `429 Too Many Requests` (Throttled): **303,563**



#### Latency Percentiles (Token Bucket)

```text
  10% in 0.0065 secs
  25% in 0.0075 secs
  50% in 0.0086 secs  <-- Median (p50): ~8.6 ms
  75% in 0.0099 secs
  90% in 0.0117 secs
  95% in 0.0132 secs
  99% in 0.0167 secs  <-- p99: ~16.7 ms

```

---

### 2. Leaky Bucket Algorithm (`/v1/leaky-resource`)

I tested the Leaky Bucket algorithm under identical load to verify constant-rate egress enforcement and bucket overflow prevention.

* **Total Requests Processed:** ~343,400
* **Throughput:** **11,445.16 Requests/Sec**
* **Status Code Distribution:**
* `200 OK` (Allowed at Leak Rate): **6,900**
* `429 Too Many Requests` (Throttled/Overflow): **336,514**



#### Latency Percentiles (Leaky Bucket)

```text
  10% in 0.0062 secs
  25% in 0.0073 secs
  50% in 0.0085 secs  <-- Median (p50): ~8.5 ms
  75% in 0.0098 secs
  90% in 0.0116 secs
  95% in 0.0131 secs
  99% in 0.0165 secs  <-- p99: ~16.5 ms

```
## 📈 Key Takeaways & Performance Observations

1. **Deterministic Concurrency:** The atomic Redis Lua script prevents race conditions under high concurrent loads (100 parallel workers), ensuring zero unauthorized requests leak through.
2. **Predictable Tail Latency:** Under heavy saturation (11,000+ RPS), 99% of all rate limit evaluations complete in **under 17 milliseconds** ($p_{99} < 17\text{ms}$).
3. **Algorithm Characteristics:**
* **Token Bucket** permits sudden burst capacity up to the bucket limit before returning `429 Too Many Requests`.
* **Leaky Bucket** strictly caps output throughput at a constant drain rate, making it ideal for protecting downstream dependencies with strict rate limits.



---

## 🧪 Running Benchmarks Locally

To reproduce these benchmarks on your local machine, run:

```bash
# Token Bucket Benchmark
hey -z 30s -c 100 http://localhost:8080/v1/token-resource

# Leaky Bucket Benchmark
hey -z 30s -c 100 http://localhost:8080/v1/leaky-resource

```
