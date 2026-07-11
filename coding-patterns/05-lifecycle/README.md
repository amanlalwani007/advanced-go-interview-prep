# Lifecycle Patterns

Start, stop, and monitor services reliably in production.

```
05-lifecycle/
├── graceful-shutdown/    # OS signal handling + context cancellation
├── health-check/         # Periodic dependency health polling
└── lifecycle-manager/    # Service registry with coordinated start/stop
```

---

## Graceful Shutdown

**File:** [`graceful-shutdown/graceful-shutdown.go`](graceful-shutdown/graceful-shutdown.go)

### What It Does

Listens for OS signals (`SIGINT`, `SIGTERM`), cancels a root context, drains in-progress work, and exits cleanly.

```go
sig := make(chan os.Signal, 1)
signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

// start services with ctx
go s1.Start(ctx, &wg)
go s2.Start(ctx, &wg)

<-sig      // block until signal
cancel()   // signal all services to stop
wg.Wait()  // wait for graceful drain
```

### Key Implementation Details

- `signal.Notify` captures OS signals. A buffered channel (size 1) prevents missing the signal during setup.
- A shared `context.WithCancel` is passed to all services. `cancel()` triggers their shutdown.
- `sync.WaitGroup` tracks in-flight work. `wg.Wait()` blocks until all services confirm shutdown.
- Combine with `context.WithTimeout` to force-stop if shutdown takes too long.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Every long-running process (HTTP servers, workers, CLIs) | Short-lived batch jobs (process exits naturally) |
| Containerised applications (Kuberentes sends SIGTERM) | Libraries called from other code |
| Processes that hold connections, files, or in-memory state | Fire-and-forget scripts |

### Real-World Scenarios

**Kubernetes pod.** During a rolling update, Kubernetes sends `SIGTERM` to the pod. The graceful shutdown handler:
1. Stops accepting new HTTP requests.
2. Drains in-flight requests (up to `http.Server.Shutdown()` timeout).
3. Closes database connections.
4. Flushes pending metrics.
5. Exits.
If shutdown takes >30s (Kubernetes `terminationGracePeriodSeconds`), the pod is force-killed.

**Message queue consumer.** On `SIGTERM`, the consumer stops pulling new messages, processes the current batch, commits offsets, and exits. No messages are lost (at-least-once) or duplicated.

### Best Practices

1. **Set a shutdown timeout.** Wrap `wg.Wait()` in a `select` with `time.After(maxShutdownTime)`.
2. **Log each phase.** "received signal", "shutting down server A", "server A stopped", "exiting".
3. **Don't force-stop before flushing critical state.** WAL, offset commits, metric batches should flush before exit.

---

## Health Check

**File:** [`health-check/health-check.go`](health-check/health-check.go)

### What It Does

Periodically runs check functions against dependencies (database, cache, upstream services). Each check is isolated, concurrent, and reports one of three states: `Healthy`, `Degraded`, `Unhealthy`.

```go
type HealthCheck struct {
    name   string
    status HealthStatus
    check  func() error
}

checker := NewHealthChecker(10*time.Second, dbCheck, cacheCheck, upstreamCheck)
go checker.Start(stop)
```

### Key Implementation Details

- Each check runs in its own goroutine to avoid one slow check blocking others.
- State transitions: consecutive failures degrade status from Healthy → Degraded → Unhealthy.
- A successful check resets to Healthy immediately.
- The checker runs on a ticker — no background goroutine leaks.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Kubernetes liveness/readiness probes | One-time health assertions in tests |
| Load balancer health endpoints | Checking things that never fail (always just return healthy) |
| Service mesh sidecars | Replacing proper monitoring (Prometheus alerts) |

### Real-World Scenarios

**Kubernetes readiness probe.** `GET /health/ready` is called every 10s. If the database check fails for 2 consecutive iterations, status becomes `Unhealthy`. Kubernetes removes the pod from Service endpoints. Traffic stops routing to the broken pod. When the DB recovers and the next check passes, the pod is re-added.

**Multi-service health dashboard.** A microservice aggregates health from 12 downstream services. Each downstream has its own health check. The aggregator returns a summary: `{"status": "degraded", "services": {"postgres": "healthy", "redis": "unhealthy", "fraud-api": "healthy"}}`. Operators see at a glance which dependency is failing.

---

## Lifecycle Manager

**File:** [`lifecycle-manager/lifecycle-manager.go`](lifecycle-manager/lifecycle-manager.go)

### What It Does

Registers multiple `Service` implementations and provides coordinated `StartAll()` / `Shutdown()`:

```go
type Service interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Name() string
}

lm := NewLifecycleManager()
lm.Register(httpServer)
lm.Register(consumer)
lm.StartAll()
// ... on signal:
lm.Shutdown(ctx)
```

### Key Implementation Details

- Services are stored in a `map[string]Service` by name.
- `StartAll()` iterates over the map and calls `Start()` on each.
- `Shutdown()` cancels the manager's root context, then calls `Stop()` on all services concurrently.
- Errors from `Start()`/`Stop()` are collected and returned.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Daemons with 3+ co-dependent services | Single-process applications |
| Applications with ordered startup requirements | When all services already follow the same context lifecycle |
| Testing service lifecycle interactions | Simple scripts |

### Real-World Scenarios

**Streaming platform daemon.** Runs 5 services: `gRPC-server`, `Kafka-consumer`, `metrics-exporter`, `profile-profiler`, `cache-warmer`. The lifecycle manager starts all 5. On `SIGTERM`, it shuts down the consumer first (commit offsets), then the gRPC server (drain requests), then the rest. If the consumer takes >10s to drain, the shutdown context times out and force-stops.

**Test harness.** Integration tests use `LifecycleManager` to spin up a test HTTP server, a test database container, and a test message queue. `lm.Shutdown()` in `t.Cleanup()` ensures clean teardown even if the test panics.
