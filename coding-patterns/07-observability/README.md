# Observability Patterns

See what your system is doing in production.

```
07-observability/
├── structured-logging/    # JSON logger with levels and context fields
└── metrics-collection/    # Counters, gauges, and histograms
```

---

## Structured Logging

**File:** [`structured-logging/structured-logging.go`](structured-logging/structured-logging.go)

### What It Does

Produces machine-parseable JSON log entries with levels, timestamps, and structured context fields:

```json
{"timestamp":"2026-07-12T02:30:00Z","level":"ERROR","message":"charge failed","service":"payments","request_id":"r_abc","amount":99.99,"error":"card_declined"}
```

```go
log := NewLogger(INFO).With("service", "payment-api")
log.Info("processing payment", "amount", 99.99, "currency", "USD")
log.Error("payment failed", "error", "insufficient funds")
```

### Key Implementation Details

- `With(key, value)` returns a new logger with immutable field context (copy-on-write).
- `log()` checks the level before formatting — DEBUG entries cost almost nothing when level is INFO.
- Timestamps are UTC RFC3339 for consistency across time zones.
- Messages use `fmt.Sprintf` for formatting — arguments are evaluated only when the log entry is created.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Every application, everywhere | Local debugging (use `fmt.Println` temporarily) |
| Production services consumed by log aggregators | Secrets and PII (log them masked or not at all) |
| Alerting and monitoring pipelines | High-cardinality values per-request (use metrics) |

### Real-World Scenarios

**Debugging production issues.** A payment is failing. DevOps queries: `{service:"payments", level:"ERROR"} | sort by timestamp desc | limit 20`. The results show the error, the request ID, the amount, and the merchant. Using that request ID, they trace through the API gateway logs and the database logs to find the root cause. Unstructured logs would require grep across multiple files with no consistent schema.

**Alerting.** A monitoring rule triggers when `level:ERROR` exceeds 10/minute for `service:checkout`. The on-call engineer receives a notification with the last 5 error entries, each with the error type and the affected user segment. Structured fields make aggregation queries trivial.

### Log Levels

| Level | Purpose | Examples |
|-------|---------|----------|
| DEBUG | Detailed diagnostic info, disabled in prod | SQL queries, HTTP request/response dumps |
| INFO | Normal operation events | Request started/completed, cache hit/miss |
| WARN | Something unexpected but not an error | Slow query, retry attempt, deprecated endpoint used |
| ERROR | Something failed and needs attention | API call failed, database connection lost, panic recovered |

### Best Practices

1. **Include request_id in every log line.** Without it, correlating logs across services is impossible.
2. **Log at the boundary.** Log requests entering and leaving your service, not every internal function call.
3. **Never log secrets.** API keys, passwords, tokens, and PII should be redacted.
4. **Use consistent field names.** `request_id` everywhere, not sometimes `reqID`, `requestId`, `Req-Id`.

---

## Metrics Collection

**File:** [`metrics-collection/metrics-collection.go`](metrics-collection/metrics-collection.go)

### What It Does

An in-memory metrics registry with three fundamental instrument types:

```go
reg := NewRegistry()

counter := reg.Counter("http_requests_total")
counter.Inc()

gauge := reg.Gauge("active_connections")
gauge.Set(42)

histogram := NewHistogram("request_duration_ms", []int64{5, 10, 25, 50, 100, 500})
histogram.Observe(47)
```

### Metric Types

**Counter** — a monotonically increasing integer. Only `Inc()` or `Add(n)`. Represents counts of events.

```
http_requests_total{method="GET",status="200"} = 150432
```

**Gauge** — a point-in-time value that can increase or decrease. Represents current state.

```
active_connections = 42
memory_usage_bytes = 847283910
```

**Histogram** — samples observations into configurable buckets, counting how many fall into each. Used for latency distributions.

```
request_duration_ms{le="5"}    = 10234
request_duration_ms{le="10"}   = 48291
request_duration_ms{le="25"}   = 89201
request_duration_ms{le="50"}   = 102934
request_duration_ms{le="100"}  = 109283
request_duration_ms{le="500"}  = 109482
request_duration_ms{le="+Inf"} = 109502
```

### Key Implementation Details

- `Counter` uses `sync/atomic.Int64` for lock-free increments on hot paths.
- `Gauge` similarly uses `atomic.Int64` — `Set` and `Load` are thread-safe.
- `Histogram` uses a slice of `atomic.Int64` counts per bucket.
- The `MetricsRegistry` uses `sync.RWMutex` — reads (reporting) are shared-locked, writes (registration) are exclusive.

### When to Use

| Type | Do Use | Don't Use |
|------|--------|-----------|
| Counter | Request counts, error counts, bytes sent | Values that can decrease (use gauge) |
| Gauge | Memory usage, queue size, connection count | Monotonic values (use counter) |
| Histogram | Request latency, response size, batch size | Single values (use gauge or counter) |

### Real-World Scenarios

**RED monitoring.** Rate (requests/second), Errors (error rate), Duration (latency percentiles). Every service tracks:
- `requests_total{endpoint,method}` (counter → rate)
- `requests_errors_total{endpoint,method}` (counter → error rate)
- `request_duration_seconds{endpoint,method}` (histogram → p50, p95, p99 latency)

An alert fires when `p99 latency > 1s` for 5 minutes. The on-call engineer checks which endpoint is affected, compares with the deploy timeline, and rolls back.

**USE monitoring.** Utilization (CPU, memory), Saturation (queue depth), Errors (failure count). Tracked per node:
- `cpu_usage_percent` (gauge)
- `memory_usage_bytes` (gauge)
- `goroutine_count` (gauge)
- `go_gc_pause_seconds` (histogram)

A high goroutine count + high GC pause suggests a goroutine leak. Metrics show which service version started leaking after the last deploy.

### Production Integration

The patterns in this directory are simplified. In production, use a battle-tested library:

| Library | Notes |
|---------|-------|
| **Prometheus client** (`prometheus/client_golang`) | De facto standard. Supports counters, gauges, histograms, summaries. Built-in HTTP handler for `/metrics`. |
| **OpenTelemetry** (`go.opentelemetry.io/otel/metric`) | Industry standard for metrics + traces + logs. Vendor-neutral. |
| **expvar** (stdlib) | Built-in. Simple but limited. No histogram support. |
