# Error Handling Patterns

Handle failures with type safety, panic safety, and clear semantics.

```
03-error-handling/
├── result-type/       # Generic Result[T] for explicit error handling
└── panic-recovery/    # Safe goroutine execution with panic capture
```

---

## Result Type (Generics)

**File:** [`result-type/result-type.go`](result-type/result-type.go)

### What It Does

Wraps a value and error into a single generic type:

```go
type Result[T any] struct {
    Value T
    Err   error
}

func Ok[T any](v T) Result[T]   { return Result[T]{Value: v} }
func Err[T any](e error) Result[T] { return Result[T]{Err: e} }
```

Usage:
```go
func divide(a, b int) Result[int] {
    if b == 0 {
        return Err[int](errors.New("division by zero"))
    }
    return Ok(a / b)
}

r := divide(10, 2)
if val, err := r.Unwrap(); err != nil {
    // handle
}
```

### Key Implementation Details

- Uses Go 1.18+ generics — `Result[T]` works with any type.
- `Ok()` and `Err()` constructors make intent clear.
- `Unwrap()` returns the classic `(T, error)` tuple for compatibility.
- `IsOk()` / `IsErr()` for boolean checks without unpacking.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Chains of operations that can fail | Simple functions (use classic `(T, error)`) |
| When you want to enforce error handling at compile time | Codebases that predate Go 1.18 generics |
| Functional-style composition pipelines | Performance-critical hot loops (allocation overhead) |

### Real-World Scenarios

**Validation pipeline.** `parseEmail → validateDomain → checkMX`. Each returns `Result[*Email]`. The chain maps over the result, threading the value through only if the previous step succeeded. A single `if err != nil` at the end handles all failure cases.

**API response unmarshalling.** An HTTP client receives JSON, parses it into `T`, and returns `Result[T]`. The caller pattern-matches: `r := fetch[User](url); if r.IsOk() { use(r.Value) }`.

### Why Not Just Return (T, error)?

- `Result[T]` can be stored in channels, slices, and maps without wrapper structs.
- Enables combinators like `.Map()`, `.OrElse()`, `.AndThen()` for functional chains.
- The type system guarantees the caller checks the error — no accidental ignoring.

---

## Panic Recovery

**File:** [`panic-recovery/panic-recovery.go`](panic-recovery/panic-recovery.go)

### What It Does

Wraps goroutine execution with a deferred `recover()` that captures panics and their stack traces without crashing the process.

```go
type SafeGo struct {
    Errors chan error
}

func (s *SafeGo) Go(fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                s.Errors <- fmt.Errorf("panic: %v\n%s", r, debug.Stack())
            }
        }()
        fn()
    }()
}
```

### Key Implementation Details

- `defer` catches panics in the spawned goroutine.
- `debug.Stack()` captures the full goroutine stack trace for debugging.
- Errors are forwarded to a channel for centralised handling.
- The original function signature is unchanged — no `recover()` boilerplate in business logic.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Background goroutines in long-lived services | Recovering from nil-pointer dereferences in business logic (fix the bug) |
| HTTP handlers / message handlers | Replacing proper error propagation |
| Third-party library code that might panic | As a general try-catch substitute |

### Real-World Scenarios

**Message queue consumer.** A consumer goroutine processes each message. One message triggers a nil-pointer panic due to a malformed payload. `SafeGo` catches the panic, logs the stack trace, and sends the message to a dead-letter queue. The consumer continues processing the next message without restarting.

**HTTP server.** A handler panics because of an unexpected nil. The recovery middleware catches it, logs the stack, returns a 500 Internal Server Error, and the server keeps running. Without recovery, the entire process would crash, dropping all in-flight requests.

### Go's Philosophy on Panics

> "In Go, panics are for programmer errors — nil pointer dereferences, out-of-bounds array access, etc. They should not be used for normal error handling."

- **Don't** use `recover()` to implement try-catch logic.
- **Do** use `recover()` at goroutine boundaries to prevent crashes from unexpected bugs.
- **Do** log the full stack trace when recovering — silently swallowing panics makes debugging impossible.

### When NOT to Recover

- If you can't guarantee the program is in a consistent state (e.g., mutex held, file locked during panic).
- If the panic indicates data corruption — sometimes crashing is the safer option to prevent cascading corruption.
