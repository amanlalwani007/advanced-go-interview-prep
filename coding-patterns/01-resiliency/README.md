# Resiliency Patterns

Protect your service from cascading failures, resource exhaustion, and unreliable dependencies.

```
01-resiliency/
├── circuit-breaker/       # Fail-fast when downstream is unhealthy
├── retry-backoff/         # Retry with exponential delay + jitter
├── rate-limiter/          # Token-bucket request throttling
├── bulkhead/              # Bounded concurrency per component
├── timeout-context/       # Deadline-enforced cancellation
└── debounce-throttle/     # Coalesce rapid-fire events
```

---

## Circuit Breaker

**File:** [`circuit-breaker/circuit-breaker.go`](circuit-breaker/circuit-breaker.go)

### What It Does

Wraps calls to an external service in a state machine:

```
          ┌───── failure threshold met ────┐
          │                                  ▼
    ┌─────────┐                        ┌─────────┐
    │  CLOSED │──────► TRIPS ─────────►│   OPEN  │
    │ (normal)│                        │(fast-fail)│
    └─────────┘                        └─────────┘
          ▲                                  │
          │     half-open succeeds           │
          │   (probe passes, count reset)    │
          │                                  │
          │         ┌──────────┐             │
          └─────────│ HALF-OPEN│◄────────────┘
                    │ (probe)  │  recovery time passed
                    └──────────┘
```

- **Closed** — normal operation. Failures are counted.
- **Open** — threshold reached. All calls fail instantly with `ErrCircuitOpen`.
- **Half-Open** — after `recoveryTime`, one probe is allowed. Success → Closed. Failure → Open again.

### Key Implementation Details

- `sync.Mutex` guards state transitions.
- `failureCount` resets on success in Closed state.
- `successCount` in Half-Open requires N consecutive successes to transition to Closed.
- `lastFailureTime` determines when the recovery window starts.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| HTTP/gRPC calls to external services | In-process function calls (no network boundary) |
| Database connection pools | Operations that must always be attempted |
| Message queue consumers | Idempotent operations better served by retry |
| Any dependency with unpredictable latency | Monolithic single-process code |

### Real-World Scenarios

**E-commerce checkout.** Payment gateway starts 500'ing. After 3 failures the circuit opens. Checkout users get "payment temporarily unavailable" in 2ms instead of hanging for 30s waiting for a timeout. The gateway gets traffic-free recovery time. Once healthy, the next probe succeeds and traffic resumes.

**Cache-aside.** Redis is under replication lag and starts timing out. Circuit breaker opens. The application falls back to reading from PostgreSQL directly (stale but available). When Redis recovers, the circuit closes and the fast path resumes.

### Failure Modes

- **Too sensitive** — brief network hiccup trips the breaker. Solution: tune threshold and recovery time.
- **Too lenient** — breaker never opens because traffic volume is low and failures recover before threshold. Solution: track failure rate ratio, not absolute count.

---

## Retry with Exponential Backoff

**File:** [`retry-backoff/retry-backoff.go`](retry-backoff/retry-backoff.go)

### What It Does

Re-runs a failed operation up to `N` attempts with exponentially increasing delays:

```
delay = min(base * 2^attempt + jitter, max_delay)
```

| Attempt | Base 100ms | With Jitter (~0-100ms) |
|---------|------------|----------------------|
| 0       | 100ms      | ~130ms               |
| 1       | 200ms      | ~240ms               |
| 2       | 400ms      | ~450ms               |
| 3       | 800ms      | ~870ms               |
| 4       | 1600ms     | ~1650ms              |

### Key Implementation Details

- **Exponential factor (`2^attempt`)** — gives the downstream service exponentially more time to recover.
- **Jitter (`rand.Float64() * base`)** — prevents thundering herd. Without jitter, N clients retry simultaneously at the same tick.
- **Cap (`max_delay`)** — prevents unbounded wait times.
- Returns the **last error** wrapped with attempt count if all attempts fail.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Transient failures (network, 503, 429) | 4xx client errors (400, 401, 403, 404) |
| Background jobs / async processing | Real-time user-facing requests (use circuit breaker instead) |
| Operations with no side-effects on retry | Non-idempotent operations without idempotency key |

### Real-World Scenarios

**API client.** Calling a rate-limited external API returns 429 "Too Many Requests". Retry with backoff gives the API server time to drain its queue. Without jitter, all your retries would arrive at the same moment the rate window resets, causing another 429.

**Database transaction.** A serialisation failure in PostgreSQL (40001) means "retry the transaction". The client backs off 50ms→100ms→200ms, and by the third attempt the conflicting transaction has committed and the retry succeeds.

### Common Pitfalls

- **No jitter** — creates thundering herd, defeating the purpose of backoff.
- **Not capping max delay** — a 32nd retry at base 100ms would wait ~2.7 billion years.
- **Retrying permanent failures** — always check status codes before retrying.

---

## Rate Limiter (Token Bucket)

**File:** [`rate-limiter/rate-limiter.go`](rate-limiter/rate-limiter.go)

### What It Does

Implements the **Token Bucket** algorithm:

- A bucket holds up to `capacity` tokens.
- Tokens are added at `rate` tokens/second.
- Each request consumes one token.
- If the bucket is empty, the request is denied.

```
  tokens ▲
  cap    │ ████████████████████░░░░░░░░░░░
         │ ████████████████████  ← refilling at rate/s
         │ ░░░░░░░░░░░░░░░░░░░░  ← consumed by requests
         └────────────────────────────────► time
```

### Key Implementation Details

- `lastRefill` tracks the last time tokens were added, computing elapsed seconds since then.
- `refill()` is called on every `Allow()` before checking tokens. This makes the limiter self-cleaning — no background goroutine needed.
- Mutex ensures thread safety; lock is held only for the refill+check window.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| API gateways | Per-user fairness (use weighted fair queuing) |
| Protecting external API keys from rate-limit bans | Distributing a fixed capacity across users (use sliding window) |
| Preventing brute-force login attacks | Throttling internal message passing |

### Real-World Scenarios

**Public API.** GitHub API allows 5000 requests/hour per token. A token bucket with `capacity=5000` and `rate=1.38` (~5000/3600) enforces this smoothly. A burst of 100 requests in 1 second all get through (bucket drains to 4900). Then requests trickle at 1.38/s.

**Login endpoint.** 5 attempts per minute per IP. Bucket size 5, refill rate 0.083/s (5/60). After 5 rapid failed logins, subsequent attempts are denied for ~1 minute until a token refills.

### Token Bucket vs Other Algorithms

| Algorithm | Pros | Cons |
|-----------|------|------|
| Token Bucket | Allows bursts, simple | Memory per key |
| Leaky Bucket | Smooth output, fixed rate | No burst support |
| Sliding Window | Fair over time, precise | More complex, more memory |
| Fixed Window | Simple, cheap | Boundary spikes (traffic rush at window reset) |

---

## Bulkhead

**File:** [`bulkhead/bulkhead.go`](bulkhead/bulkhead.go)

### What It Does

Limits concurrent execution of a group of tasks using a **buffered channel as a semaphore**:

```
                  ┌──────────────────────────┐
    tasks ───────►│  channel (buffered=3)    │──► goroutine pool (max 3)
                  │  [slot1] [slot2] [slot3] │
                  └──────────────────────────┘
    rejected tasks ──► return false immediately
```

- `ch <- struct{}{}` blocks only if all slots are full.
- `select` with `default` enables non-blocking rejection.

### Key Implementation Details

- Buffer size = max concurrency limit.
- `wg.Add(1)` / `wg.Done()` tracks completions for graceful draining.
- `Execute()` returns `bool` — `true` if accepted, `false` if rejected.
- Deferred `<-ch` in the goroutine releases the slot on completion.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Isolating different workload types (fast vs slow) | Single type of homogeneous work (use worker pool) |
| Preventing a noisy neighbour from starving shared goroutines | Replacing proper request queuing |
| Protecting external resources that have their own concurrency limits | Trivial operations that complete instantly |

### Real-World Scenarios

**gRPC server.** Two RPC handlers: `GetUserProfile` (fast, 2ms) and `GenerateReport` (slow, 30s). Without bulkheads, 4 concurrent report requests consume all goroutines. User profiles queue behind reports. With a bulkhead of 2 for reports, at most 2 reports run simultaneously, leaving the rest of the pool for profile requests.

**Database connection pool.** A search microservice shares 10 connections. The "analytics query" feature gets a bulkhead of 2, the "user-facing search" gets 8. A runaway analytics query cannot starve production traffic.

---

## Timeout with Context

**File:** [`timeout-context/timeout-context.go`](timeout-context/timeout-context.go)

### What It Does

Uses Go's `context.Context` to enforce a wall-clock deadline on any operation:

```go
ctx, cancel := context.WithTimeout(parent, 1*time.Second)
defer cancel()

select {
case result := <-operation(ctx):
    // success
case <-ctx.Done():
    // ctx.Err() == context.DeadlineExceeded
}
```

### Key Implementation Details

- `context.WithTimeout` creates a context that auto-cancels after the duration.
- Always `defer cancel()` to release resources even if the operation completes before the timeout.
- Check `ctx.Done()` in a `select` to make your operation cooperative with cancellation.
- Propagate the context through the entire call chain — never store it in a struct.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Every external I/O operation (HTTP, DB, RPC, file) | CPU-bound computation (use `context.WithTimeout` with separate goroutines) |
| User-facing request handlers | Fire-and-forget background tasks (use a different approach) |
| Operations with SLAs | Operations with no upper bound |

### Real-World Scenarios

**Database query.** A report query might run for minutes on large datasets. Enforce a 30s context timeout. If the DB is under load and the query doesn't complete, the goroutine stops waiting, the connection returns to the pool, and the user gets a timeout response instead of a hung connection.

**HTTP client.** An API gateway calls three upstream services in parallel, each with a 2-second context timeout. One upstream is slow. After 2s, that branch cancels, the gateway returns a partial response (or retries), and the slow upstream's goroutine is released.

### Best Practices

1. **Always defer cancel().** Even if the operation completes, cancel frees resources.
2. **Timeout + Retry.** Combine with retry backoff: each retry attempt gets its own fresh context with timeout.
3. **Timeout + Circuit Breaker.** Circuit breaker opens when timeouts are frequent. This prevents the timeout itself from becoming a bottleneck (waiting 2s for a dead service).

---

## Debounce & Throttle

**File:** [`debounce-throttle/debounce-throttle.go`](debounce-throttle/debounce-throttle.go)

### What It Does

Two event-frequency control patterns:

**Debounce** — coalesces a burst of calls into a single execution after a quiet period.

```
calls:   | █ █ █ █   |          | █   |
         t=0        200ms       t=1s
fires:   |           |█         |     |█
                    fires once         fires once
                    after 200ms        after 200ms
                    quiet period       quiet period
```

**Throttle** — ensures execution at most once per interval, dropping intermediate calls.

```
calls:   | █ █ █ █ █ █ █ █ █ █ |
fires:   | █       █       █    |
         0ms     300ms    600ms
```

### Key Implementation Details

**Debounce:** `time.AfterFunc(d, fn)` on each call, stopping the previous timer. Only the last call in the burst triggers.

**Throttle:** Tracks `time.Since(last)`. If elapsed < interval, skip. Otherwise, execute and update `last`.

### When to Use

| Pattern | Do Use | Don't Use |
|---------|--------|-----------|
| Debounce | Search-as-you-type, auto-save, window resize | Real-time updates (use throttle) |
| Throttle | UI scroll handlers, progress bars, rate-limited API calls | One-shot events (no frequency control needed) |

### Real-World Scenarios

**Debounce — Search bar.** User types "machine learning" in 500ms. Without debounce, 15 API calls fire: `/search?q=m`, `/search?q=ma`, ..., `/search?q=machine+learning`. With 300ms debounce, only the final query fires. Saved 14 API calls, 14 DB queries.

**Throttle — Progress bar.** A file upload emits progress events at sub-millisecond granularity. Throttling at 100ms means the UI updates at most 10 times/second. The browser doesn't choke on rendering updates, and the user still sees smooth progress.
