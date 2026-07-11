# General Patterns

Utility patterns useful across any Go codebase.

```
09-general/
├── context-values/    # Typed context value propagation
└── sync-pool/         # Object recycling for hot-path allocations
```

---

## Context Values

**File:** [`context-values/context-values.go`](context-values/context-values.go)

### What It Does

Propagates request-scoped values through `context.Context` using typed getter/setter functions:

```go
type contextKey string

const UserIDKey contextKey = "user_id"

func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, UserIDKey, userID)
}

func UserIDFrom(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(UserIDKey).(string)
    return v, ok
}
```

### Key Implementation Details

- **Unexported key type** (`contextKey`) prevents collisions. External packages cannot accidentally overwrite your context values because they can't access the unexported type.
- **Getter returns `(T, bool)`** — the boolean signals whether the value exists, letting the caller distinguish between "not set" and "set to zero value".
- **Helper function `ExtractLogContext`** aggregates multiple values into a struct for convenient logging.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Request IDs, trace IDs, user IDs, auth tokens | Function arguments that are always required |
| Values that cross API boundaries (middleware → handler → service) | Mutable values (context values must be immutable) |
| Cross-cutting concerns (logging, tracing, auth metadata) | Business logic parameters (pass them directly) |

### Real-World Scenarios

**HTTP request tracing.** A gateway middleware extracts `X-Request-ID` from the HTTP header and stores it in the context via `WithRequestID`. All downstream layers (service, repository, client) can access it via `RequestIDFrom(ctx)` without the ID being threaded through every function signature. Every log line and metric label includes the request ID for correlation.

**Multi-tenant auth.** An auth middleware validates the JWT, extracts `tenant_id` and `user_id`, and stores them in the context. Downstream code calls `TenantIDFrom(ctx)` and `UserIDFrom(ctx)` to scope database queries to the correct tenant. Adding a new field (e.g., `role`) requires no changes to any handler signature.

### Best Practices

1. **Never store mutable values in context.** Context values must be immutable — create new entries with `context.WithValue`, never modify in place.
2. **Keep context keys in a central place.** All keys and getter/setter functions in one file per package.
3. **Prefer exported getters, unexported keys.** The key type (`contextKey`) is unexported to prevent collisions. The getter/setter are exported for use by other packages.
4. **Document what values are expected.** `WithUserID` documents that the context should contain a user ID in the auth format.

### What NOT to Store in Context

- **Optional parameters** — pass them as function arguments.
- **Database connections** — too heavy for context, pass as explicit dependencies.
- **Mutable values** — context is immutable by convention.
- **Large data** — context values should be small identifiers (strings, IDs), not full objects.

---

## Sync Pool

**File:** [`sync-pool/sync-pool.go`](sync-pool/sync-pool.go)

### What It Does

Reuses allocated objects across multiple goroutines, reducing GC pressure:

```go
var bufferPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func formatMessage(parts []string) string {
    buf := bufferPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufferPool.Put(buf)
    // use buf...
    return buf.String()
}
```

### How It Works

```
goroutine 1:  Get() → buf[A] → use → Put() → pool
goroutine 2:  Get() → buf[A] → use → Put() → pool
goroutine 3:  Get() → pool.New() → buf[B] → use → Put() → pool
```

- `Get()` retrieves an existing object from the pool or calls `New` if the pool is empty.
- `Put()` returns the object to the pool for reuse.
- Objects in the pool can be garbage collected at any time — the pool is a cache, not a persistent store.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Hot paths allocating many short-lived objects | Code that runs infrequently (allocation overhead is negligible) |
| `bytes.Buffer`, `strings.Builder` in serialisation code | Objects that hold external resources (file handles, network connections) |
| Protobuf marshalling/unmarshalling | Objects with complex reset logic (just allocate new ones) |

### Real-World Scenarios

**JSON serialisation.** An API gateway serialises 50,000 JSON responses/second. Each response uses a `bytes.Buffer` for marshalling. Without `sync.Pool`, each buffer is heap-allocated and garbage collected, causing GC to run at 15% CPU. With `sync.Pool`, buffers are recycled, GC drops to 3% CPU.

**Log formatting.** A high-throughput logger formats log entries using `bytes.Buffer`. With a pool of buffers, the logger avoids allocating a new buffer per entry. At 100,000 logs/second, this saves ~800 MB/s of allocation.

### Common Pitfalls

1. **Forgetting to `Reset()`.** A buffer from the pool contains previous data. Always reset before use.
2. **Putting the wrong type.** `Put()` accepts `any` — type assertion in `Get()` panics if the wrong type is stored.
3. **Assuming objects survive GC.** The pool can be drained during a GC cycle. Don't rely on the pool as a persistent cache.
4. **Putting objects after `Reset()` but before use.** Always `Get → use → Put` in the same scope. Never store pooled objects across goroutines.
5. **Overhead for small objects.** For objects smaller than ~100 bytes, the pool overhead may exceed the allocation cost. Profile first.

### When NOT to Use

- Objects that hold network connections, file handles, or locks.
- Code paths that run once or infrequently (initialisation, cron jobs).
- When profiling shows GC is not the bottleneck.
