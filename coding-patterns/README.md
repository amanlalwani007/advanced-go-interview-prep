# Go Coding Patterns

Production-grade Go patterns organized by category. Each directory contains a standalone `package main` with runnable code.

```bash
GO111MODULE=off go run ./coding-patterns/01-resiliency/circuit-breaker/
GO111MODULE=off go test ./coding-patterns/08-testing/table-driven-tests/
```

---

## 01 — Resiliency

Protect your system from failure cascades, overload, and unreliable dependencies.

### circuit-breaker

**What.** Wraps calls to an external service. Tracks failures in a state machine: **Closed** (normal) → **Open** (failing, fast-fail) → **Half-Open** (probe to check recovery). After a threshold of failures, the circuit opens and all calls fail instantly without hitting the downstream. After a recovery timeout, a probe is allowed — if it succeeds, the circuit closes.

**When.** Any time you call a remote service (HTTP/gRPC/database). Without it, a dying dependency drags down your entire service with cascading timeouts and resource exhaustion.

**Real world.** E-commerce checkout calls the payment gateway. If the gateway starts returning 500s, the circuit opens in ~3 failures. Subsequent checkout requests fail in microseconds instead of hanging for 30s. The gateway gets breathing room to recover. Once healthy, traffic resumes.

### retry-backoff

**What.** Re-runs a failed operation with exponentially increasing delay between attempts. Includes **jitter** (randomised offset) to avoid thundering-herd when many clients retry simultaneously: `delay = min(base * 2^attempt + random_jitter, max_delay)`.

**When.** Operations that fail due to transient conditions — network blips, rate limits (429), temporary unavailability (503). Never retry on 4xx client errors (400, 401, 403, 404).

**Real world.** A background job fetches data from an external API. If the API returns 429 (rate limited), the retry waits 100ms → 200ms → 400ms → 800ms, each with jitter. The API gets space to drain its queue. If all 5 attempts fail, the error surfaces to the monitoring system.

### rate-limiter

**What.** **Token Bucket** algorithm. A bucket holds `N` tokens. Each request consumes one token. Tokens refill at a fixed rate per second. When the bucket is empty, requests are denied.

**When.** Protect your own resources from abuse, or conform to upstream API rate limits. Essential in any public API gateway.

**Real world.** A social-media API allows 1000 requests/minute per API key. The rate limiter lets 1000 tokens fill the bucket. A burst of 100 requests gets through immediately (consuming 100 tokens). Subsequent requests are throttled until tokens refill at ~16.6 tokens/second.

### bulkhead

**What.** Isolates resources into pools with fixed concurrency limits using a buffered channel as a semaphore. If all slots are occupied, new tasks are immediately rejected instead of queued.

**When.** When different parts of your system share a thread pool or goroutine pool and one noisy component could starve others. Named after ship hull compartments — if one compartment floods, the ship stays afloat.

**Real world.** A gRPC server handles both latency-sensitive user-facing queries and slow batch-report generation. Both share a goroutine pool. The batch jobs (bulkhead of 3) cannot consume all goroutines, so user queries always get CPU time.

### timeout-context

**What.** Uses `context.WithTimeout` to enforce a maximum wall-clock duration for an operation. If the deadline passes, `ctx.Done()` fires and the operation receives `context.DeadlineExceeded`.

**When.** Every external call — database queries, HTTP requests, RPCs, file I/O. Without a timeout, a hung dependency leaks goroutines and memory forever.

**Real world.** A search service calls an indexing backend with a 2-second timeout. The backend's disk is slow today. After 2s, the context cancels, the goroutine stops waiting, and the user sees a degraded response instead of a spinning loader for 60s.

### debounce-throttle

**What.** **Debounce** — coalesces rapid-fire calls into one execution after a quiet period. The last invocation wins. **Throttle** — guarantees execution at most once per time interval, dropping intermediate calls.

**When.** Debounce for input events (search-as-you-type, save-on-idle). Throttle for progress updates, scroll handlers, or UI animation frames where you must not overwhelm the consumer.

**Real world.** A search bar sends API requests on every keystroke. Debouncing with 300ms delay means only the final keystroke triggers the API call. A progress bar reads task status — throttling at 1Hz means the UI updates at most once per second regardless of how many status events fire.

---

## 02 — Concurrency

Leverage goroutines and channels safely and predictably.

### worker-pool

**What.** Fixed number of goroutines read from a shared jobs channel, process, and write to a results channel. The pool bounds parallelism.

**When.** Processing a large batch of independent work items where unbounded goroutines would overwhelm memory or downstream services.

**Real world.** A media transcoder receives 10,000 videos. A pool of 4 workers pulls from a Redis queue, transcodes each video, and writes the result to S3. No more than 4 transcodes run simultaneously, keeping CPU and memory predictable.

### fan-out-fan-in

**What.** **Fan-out** — distribute a single input channel across multiple processing goroutines. **Fan-in** — merge multiple output channels into one using `sync.WaitGroup`.

**When.** Data-parallel workloads where the same computation can be split across goroutines to saturate CPU cores.

**Real world.** An image resizer takes one input channel of raw images, fans out to 4 goroutines that each resize an image, and fans in to one output channel of resized results. With 8 CPU cores, throughput is ~4x sequential.

### pipeline

**What.** Chain of stages connected by channels. Each stage is a function `func(<-chan T) <-chan U`. A `Pipeline(...)` compositor strings stages together.

**When.** Multi-step processing where each stage can run concurrently with bounded buffering between stages. Enables clean separation of concerns.

**Real world.** A log processor: `reader → parse → filter → enrich → batch-write`. The reader reads 10MB/s from disk; the parser extracts JSON fields; the filter drops DEBUG entries; the enricher adds hostname; the batch-writer flushes 1000 events to Elasticsearch. Each stage runs in its own goroutine, pipeline latency = slowest stage, not sum of all stages.

### or-channel

**What.** Recursive select that returns a single `done` channel when **any** of the input channels close. Uses reflection-free recursive select for arbitrary N channels.

**When.** When you need to wait on multiple cancellation signals simultaneously (e.g., a context timeout OR a manual stop signal OR a parent goroutine failure).

**Real world.** A sidecar proxy watches three shutdown signals: `SIGTERM` from the OS, a health-check failure signal from the main process, and a watch-dog timeout. `or(ch1, ch2, ch3)` returns as soon as any fires, triggering graceful shutdown.

### tee-channel

**What.** Splits one input channel into two identical output channels. Each value sent to the input is received by **both** outputs.

**When.** Broadcasting a stream of events to multiple independent consumers.

**Real world.** A trade feed receives 10,000 trades/second. One branch of the tee goes to the risk-analysis engine, the other to the real-time price display. Both see every trade in near-real-time without interfering.

### fan-in

**What.** Merge multiple input channels into a single output channel. Blocks until all inputs are exhausted, then closes the output.

**When.** Aggregating results from multiple independent producers into one consumer stream.

**Real world.** Three Kafka partitions are consumed by three goroutines, each with its own channel. `fanIn(ch1, ch2, ch3)` merges them into one channel consumed by a single deduplication and write stage.

---

## 03 — Error Handling

Handle failures with control and type safety.

### result-type

**What.** A generic `Result[T]` type that wraps a value and an error. Provides `IsOk()`, `IsErr()`, `Unwrap()` for ergonomic handling. Similar to Rust's `Result<T, E>`.

**When.** When you want to enforce error checking at the type level, or compose multiple operations that can fail without deep nesting of `if err != nil`.

**Real world.** A validation pipeline: `parse → validate → persist`. Using `Result[*User]`, each stage returns a result that the next stage pattern-matches on, keeping the happy path flat and errors explicit.

### panic-recovery

**What.** A `SafeGo` wrapper that spawns goroutines with a `recover()` deferred handler. Panics are caught, logged with stack traces, and forwarded to an error channel instead of crashing the process.

**When.** Any goroutine you spawn where a panic is unacceptable — background workers, HTTP handlers, message consumers. Go's philosophy is "don't panic in libraries", but operations that spawn goroutines should guard against panics.

**Real world.** A message queue consumer spawns a goroutine per message. If one message triggers a nil-pointer panic (bug in deserialization), the `SafeGo` handler catches the panic, logs the stack trace, and sends a Dead-Lettered message to the DLQ. The consumer keeps running.

---

## 04 — Structural

Compose and configure objects cleanly.

### functional-options

**What.** A variadic constructor pattern where options are functions that mutate the struct: `type Option func(*Server)`. Callers pass only the options they need. Defaults are set first, then overridden.

**When.** When constructing an object with many (10+) optional configuration fields. Avoids constructor overloads, builder boilerplate, and positional-argument misery.

**Real world.** An HTTP client library: `NewClient("https://api.example.com", WithTimeout(5s), WithRetries(3), WithTracing(true))`. Users configure only what matters to them; everything else gets sensible defaults.

### middleware

**What.** Wraps a `Handler` inside another `Handler`. Multiple middlewares compose via `Chain(handler, logging, recovery, auth)`. Executed from outermost to innermost — like an onion.

**When.** Cross-cutting concerns that must wrap every operation: logging, metrics, auth, rate limiting, recovery, tracing.

**Real world.** A Go HTTP server: every request passes through `Recovery → Logging → Auth → RateLimit`. The chain is declared once and wrapped around the router. Adding a new concern (e.g., request ID injection) is a one-line addition to the chain.

### builder

**What.** Method-chaining pattern where each setter returns `*Builder`, enabling fluent construction: `Select("id").From("users").Where("active=1").Build()`.

**When.** When constructing complex, multi-step objects (queries, protobuf messages, test fixtures) where readability matters more than conciseness.

**Real world.** A SQL query builder for tests: `Select("id", "name").From("users").Where("age > ?", 18).OrderBy("name").Limit(10).Build()` produces `SELECT id, name FROM users WHERE age > ? ORDER BY name LIMIT 10`. Easy to read, hard to malform.

---

## 05 — Lifecycle

Start, stop, and monitor services reliably.

### graceful-shutdown

**What.** Listens for OS signals (`SIGINT`, `SIGTERM`), cancels a root context, drains in-flight work via `sync.WaitGroup`, and exits cleanly.

**When.** Every long-running process — HTTP servers, workers, CLIs. Without it, `kill -9` leaves connections open, messages unprocessed, and state corrupted.

**Real world.** A Kubernetes pod receives `SIGTERM` during rolling update. The graceful shutdown handler initiates context cancellation, the HTTP server stops accepting new requests, and in-flight requests get up to 30s to complete before the process exits.

### health-check

**What.** Periodically runs check functions against dependencies (DB, cache, upstream services). Each check is concurrent. A `HealthChecker` orchestrates checks on a ticker and reports results.

**When.** Every service that reports readiness/liveness to Kubernetes, load balancers, or service meshes.

**Real world.** A payment-service health check pings PostgreSQL, Redis, and the fraud-detection gRPC endpoint every 10s. If the DB fails 2 consecutive checks, status becomes `Unhealthy`. Kubernetes stops routing traffic, removing the pod from the service endpoint.

### lifecycle-manager

**What.** Registers multiple `Service` instances with `Start()/Stop()/Name()`. `StartAll()` boots them in any order; `Shutdown()` cancels the shared context and drains all services concurrently with a deadline.

**When.** When you have multiple components that need ordered startup and parallel shutdown — typical in daemons, agents, and multi-server processes.

**Real world.** A streaming platform runs `gRPC-server`, `consumer-group`, `metrics-exporter`, `profile-profiler`. On startup, all four start. On `SIGTERM`, the lifecycle manager shuts all four down with a 10-second deadline. If the consumer takes too long draining, it's force-stopped.

---

## 06 — Messaging

Reliable async communication patterns.

### pub-sub

**What.** In-memory publish-subscribe broker. Subscribers register channels by topic. Publishers send events to all subscriber channels for that topic. Supports unsubscribe.

**When.** In-process event-driven communication: emit events from one component and react in another without direct coupling.

**Real world.** Order-processing service: when `OrderService.Create()` succeeds, it publishes `"order.created"`. `EmailService` (subscribed to `"order.created"`) sends a confirmation email. `InventoryService` (same topic) reserves stock. Both react independently. Adding a third subscriber needs zero changes to existing code.

### idempotency-key

**What.** Stores a deduplication mapping of `key → result` with TTL. Before processing, checks if the key was already seen; if so, returns the stored response instead of processing again.

**When.** Any operation that must happen at most once: payment charges, order creation, webhook handling. Network retries can cause the same request to arrive twice.

**Real world.** A payment API: client sends `POST /charge` with `Idempotency-Key: uuid-123`. The first request processes and stores `"txn_98765"`. A network retry sends the same key again. The idempotency store returns `"txn_98765"` without charging the card twice.

---

## 07 — Observability

See what your system is doing.

### structured-logging

**What.** JSON-formatted logger with levels (DEBUG/INFO/WARN/ERROR), field context via `With(key, value)`, and timestamp for every entry. Structured fields make logs machine-parseable.

**When.** Every application. JSON logs are queryable by log aggregators (ELK, Loki, CloudWatch). Unstructured `fmt.Println` is not.

**Real world.** A payment service logs `{"level":"ERROR","message":"charge failed","service":"payments","request_id":"r_abc","amount":99.99,"error":"card_declined"}`. In production, DevOps queries `level:ERROR AND service:payments` to find failures, then pivots by `error` to see the most common decline reasons.

### metrics-collection

**What.** In-memory metrics registry with `Counter`, `Gauge`, and `Histogram`. Thread-safe with `sync/atomic`. Counters are monotonic; gauges are point-in-time values; histograms track distributions.

**When.** Every production service to track request rates, error rates, latency percentiles, and resource usage.

**Real world.** A web server exposes `http_requests_total{method="POST",status="200"}` as a counter. `active_connections` as a gauge. `request_duration_seconds` as a histogram with buckets [0.01, 0.05, 0.1, 0.5, 1, 5]. Prometheus scrapes these every 15s. Alerts fire when p99 latency exceeds 1s for 5 minutes.

---

## 08 — Testing

Write tests that are readable, exhaustive, and maintainable.

### table-driven-tests

**What.** A slice of test cases (`[]struct{name, input, want, wantErr}`) iterated with `t.Run`. Each sub-test has a descriptive name and runs independently.

**When.** Every unit test for functions with multiple input/output combinations. The pattern is Go idiomatic and built into the standard library.

**Real world.** Testing a `ValidateEmail(s string) error` function: 12 cases — valid emails, missing @, missing domain, too long, Unicode, etc. Adding a new case is one line in the table. Failures show `--- PASS: TestValidateEmail/valid_professional` or `--- FAIL: TestValidateEmail/missing_at`.

### test-helpers

**What.** Custom test utilities: **fakes** (in-memory implementations of interfaces), **helpers** (functions that call `t.Helper()` for clean error reporting), and cleanup via `t.Cleanup()`.

**When.** When your tests need dependency substitution or share setup/teardown logic. Fakes are faster and more deterministic than mocks generated by frameworks.

**Real world.** A `UserService` depends on `UserRepository` interface. In tests, a `fakeUserRepository` with an in-memory map replaces the real Postgres implementation. Tests run in milliseconds, need no database, and can inject controlled failures.

---

## 09 — General

Utility patterns useful across any Go codebase.

### context-values

**What.** Typed getter/setter functions for `context.Context` using unexported `contextKey` types (not raw strings). Type-safe, collision-free, and documented via the getter function.

**When.** Propagating request-scoped data through layers without adding parameters to every function signature: request IDs, user IDs, trace IDs, auth tokens.

**Real world.** An HTTP gateway extracts `X-Request-ID` and `X-User-ID` from headers, stores them in the context via `WithRequestID` / `WithUserID`. Downstream business logic calls `ExtractLogContext(ctx)` to include these in every log line and metrics label without explicit plumbing.

### sync-pool

**What.** `sync.Pool` caches allocated-but-idle objects for reuse, reducing GC pressure. Gets a buffer from the pool; uses it; puts it back when done.

**When.** When you allocate many short-lived objects of the same type in hot paths — especially `bytes.Buffer`, `strings.Builder`, or protobuf structs. Only beneficial under high allocation rates (> tens of thousands per second).

**Real world.** A JSON serializer that handles 50,000 requests/second. Each request allocates a `bytes.Buffer` for marshaling. Using `sync.Pool` to recycle buffers reduces GC CPU usage from 15% to 3%.
