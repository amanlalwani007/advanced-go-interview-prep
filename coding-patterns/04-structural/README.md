# Structural Patterns

Compose and configure objects cleanly with idiomatic Go.

```
04-structural/
├── functional-options/   # Variadic config functions for clean constructors
├── middleware/            # Chainable wrapper functions
└── builder/              # Fluent method-chaining API
```

---

## Functional Options

**File:** [`functional-options/functional-options.go`](functional-options/functional-options.go)

### What It Does

A constructor pattern where optional configuration is expressed as functions that mutate the target struct:

```go
type Server struct {
    host    string
    port    int
    timeout time.Duration
    maxConn int
}

type Option func(*Server)

func WithPort(p int) Option {
    return func(s *Server) { s.port = p }
}

func NewServer(host string, opts ...Option) *Server {
    s := &Server{host: host, port: 8080, timeout: 30 * time.Second, maxConn: 100}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage:
s := NewServer("0.0.0.0", WithPort(9090), WithTimeout(5*time.Second))
```

### Key Implementation Details

- `Option` is a function type: `type Option func(*Server)`.
- The constructor applies default values first, then iterates over options.
- Options are applied in order — later options override earlier ones.
- No required changes to the struct or constructor when adding new options.
- The pattern is fully backward-compatible: adding a new option function doesn't break existing callers.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Structs with 3+ optional fields | Structs with 0-1 optional fields (just use a simple constructor) |
| Libraries where API stability matters | Internal-only code where refactoring is cheap |
| When different callers need different configurations | When configuration is always the same (use a single constructor) |

### Comparison with Alternatives

| Pattern | Pros | Cons |
|---------|------|------|
| Functional Options | Backward compatible, no boilerplate per caller, clear at call site | More code in constructor |
| Config struct parameter | Simple, familiar | Harder to extend (exports config struct), callers must pass empty struct |
| Builder pattern | Very readable with fluent API | More code, caller must use builder object |
| Parameter object | Clean for many params | Same breaking-change issues as config struct |

### Real-World Scenarios

**HTTP server library.** `NewServer(WithAddr(":8080"), WithTLS(cert, key), WithTimeout(30*time.Second), WithMaxHeaderBytes(1<<20))`. The library author can add `WithCompression()` in v2 without breaking any v1 callers.

**Database connection pool.** `NewPool(WithMaxOpen(25), WithMaxIdle(10), WithConnMaxLifetime(5*time.Minute), WithLogger(logger))`. Users configure only what they care about. The rest stays at sensible defaults.

---

## Middleware

**File:** [`middleware/middleware.go`](middleware/middleware.go)

### What It Does

Wraps a `Handler` with cross-cutting behaviour. Middlewares are composed into a chain:

```go
type Handler func(string) string
type Middleware func(Handler) Handler

func Chain(h Handler, middlewares ...Middleware) Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}

// Execution order: Logging → Recovery → handler
h := Chain(hello, Logging, Recovery)
```

Execution flows like an onion:

```
Request
  │
  ▼
Logging Middleware (outermost)
  │
  ▼
Recovery Middleware
  │
  ▼
handler (core)
  │
  ▼
Recovery Middleware
  │
  ▼
Logging Middleware
  ▼
Response
```

### Key Implementation Details

- `Chain` composes from right to left: the first middleware in the argument list is the outermost.
- Each middleware receives the next handler and returns a wrapped version.
- Pre/post logic is achieved by executing code before/after calling `next()`.
- The handler type is generic enough to wrap any function signature.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Cross-cutting concerns (logging, metrics, auth, recovery) | One-off wrappers (just call directly) |
| HTTP/gRPC handler chains | Operations where the wrapper is always the same (inline it) |
| Pluggable behaviour that may be added/removed per route | Logic that must run conditionally based on request body |

### Real-World Scenarios

**HTTP API.** Every route needs: `RequestID → Logging → Auth → RateLimit → handler`. The chain is defined once and applied to the router. Adding a new middleware (e.g., `LatencyTracking`) is a one-line addition. Removing auth for the health-check endpoint means using a different chain.

**gRPC interceptor.** Unary and stream interceptors are middlewares. `ChainUnaryInterceptors(logging, auth, validator, metrics)` wraps every gRPC handler. When a new concern surfaces (distributed tracing), it's added to the chain without touching a single handler.

---

## Builder

**File:** [`builder/builder.go`](builder/builder.go)

### What It Does

Method-chaining pattern where each setter returns `*Builder`, enabling fluent construction:

```go
query := Select("id", "name", "email").
    From("users").
    Where("active = true").
    Where("created_at > '2024-01-01'").
    OrderBy("name").
    Limit(10).
    Build()

// Result:
// SELECT id, name, email FROM users WHERE active = true AND created_at > '2024-01-01' ORDER BY name LIMIT 10
```

### Key Implementation Details

- Each method returns `*QueryBuilder` to enable chaining.
- The final `Build()` method produces the result and can validate the final state.
- Intermediate state is accumulated in the builder struct.
- The builder is not safe for concurrent use (caller should create per-request builders).

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Constructing complex objects with many optional parts | Simple constructors with few parameters |
| Test fixture builders | Immutable objects (use functional options) |
| SQL query builders, HTTP request builders, protobuf builders | Performance-critical path (builder promotes heap allocation) |

### Real-World Scenarios

**Test fixture builder.** A `UserBuilder` with `.WithName("Alice").WithEmail("alice@test.com").WithRole("admin").Build()` creates test data with sensible defaults overridden only where relevant for the specific test. Reduces test setup boilerplate by 60%.

**HTTP request builder.** `NewRequest().WithMethod("POST").WithURL("...").WithJSON body).WithHeader("Authorization", bearer).Build()` produces a complete `*http.Request`. Defaults handle URL encoding, content type, and compression without the caller thinking about them.
