# Testing Patterns

Write tests that are thorough, readable, and maintainable.

```
08-testing/
├── table-driven-tests/    # Go-idiomatic test case lists
└── test-helpers/          # Fakes, helpers, and cleanup utilities
```

---

## Table-Driven Tests

**File:** [`table-driven-tests/calculator.go`](table-driven-tests/calculator.go), [`table-driven-tests/calculator_test.go`](table-driven-tests/calculator_test.go)

### What It Does

Organises test cases as a slice of structs and iterates them with `t.Run`:

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {name: "positive", a: 2, b: 3, want: 5},
        {name: "negative", a: -1, b: 1, want: 0},
        {name: "zero", a: 0, b: 0, want: 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Add(tt.a, tt.b); got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}
```

### Key Implementation Details

- Each row is a complete test case with `name`, inputs, and expected outputs.
- `t.Run(name, fn)` creates a sub-test — each case runs independently with its own `*testing.T`.
- Sub-test names show in output: `--- PASS: TestAdd/positive`, `--- FAIL: TestAdd/negative`.
- Table includes the `wantErr` field for error-testing functions.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Any function with multiple input/output combinations | Functions with complex setup (use separate tests) |
| Edge case coverage (empty, nil, boundary, error) | One-off integration scenarios |
| Regression tests (add a row for each bug found) | Tests requiring unique assertions per case |

### Real-World Scenarios

**Input validation.** `ValidateEmail(s string) error` has 15 cases: valid emails, missing @, multiple dots, too long, Unicode, empty string, SQL injection attempt. Each case is a one-line addition to the table. When a new validation rule is added, the table grows by 2-3 rows.

**Business logic.** `CalculateDiscount(order *Order) (float64, error)` has 20 cases: no discount, 10% for orders >$100, 20% for VIP with coupon>10%, expired coupon, overlapping promotions. The test table makes it obvious which combinations are covered and which are missing.

### Best Practices

1. **Name cases descriptively.** `"nil_input"`, `"negative_amount"`, `"expired_coupon"`. The name appears in test output — make it readable.
2. **Always include edge cases.** Empty input, zero value, nil, max int, boundary values.
3. **Add a row for every bug fix.** When you fix a bug, add a test case that reproduces it. The table grows but never shrinks — regression coverage increases over time.
4. **Use `t.Cleanup` for teardown.** Defer runs at function scope, not sub-test scope. `t.Cleanup()` runs when the sub-test finishes.

---

## Test Helpers (Fakes, Cleanup, Error Helpers)

**File:** [`test-helpers/testhelpers_test.go`](test-helpers/testhelpers_test.go)

### What It Does

Provides reusable test infrastructure: fake implementations of interfaces, helper functions for common setup/teardown, and `t.Helper()` for clean error locations.

### Fake Implementations

```go
type fakeUserService struct {
    users map[int]*User
    err   error
}

func (s *fakeUserService) GetUser(id int) (*User, error) {
    if s.err != nil { return nil, s.err }
    u, ok := s.users[id]
    if !ok { return nil, fmt.Errorf("not found") }
    return u, nil
}
```

- Fakes are in-memory implementations of interfaces.
- They are deterministic (no randomness, no network).
- They support injecting failures (`s.err`) for error-path testing.

### Test Helpers

```go
func withTimeout(t *testing.T, d time.Duration) context.Context {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), d)
    t.Cleanup(cancel)
    return ctx
}
```

- `t.Helper()` marks the function as a test helper — failures are reported at the **caller** line, not inside the helper.
- `t.Cleanup()` schedules teardown — runs even if the test panics.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Fakes for interfaces with complex/messy real implementations | Mocking every single interface (make simple implementations in the test file) |
| Setting up test servers, databases, or files | Using fakes when the real implementation is faster and simpler (e.g., `bytes.Buffer`) |
| Common assertion helpers | Over-abstracting test setup (tests should be readable) |

### Real-World Scenarios

**Testing failure paths.** A `UserService.GetUser(id)` should return a specific error when the database is unreachable. The fake `UserRepository` has an `err` field. Setting `fake.Err = ErrDatabaseDown` in a test verifies that the service propagates the error correctly, wraps it with context, and doesn't panic.

**Integration test setup.** `SetupTestDB(t *testing.T) *sql.DB` creates a temporary PostgreSQL database, runs migrations, and registers `t.Cleanup` to drop it. The test function only writes business logic. The helper handles all the boilerplate.

### Fakes vs Mocks vs Stubs

| Approach | What It Is | When to Use |
|----------|-----------|-------------|
| **Fake** | Working implementation with in-memory storage | When the real implementation is slow, non-deterministic, or unavailable |
| **Mock** | Pre-programmed expectations and assertions (`testify/mock`, `gomock`) | When you need to verify call order, call count, or exact arguments |
| **Stub** | Returns canned responses with no logic | When you only need a controlled return value |

### Golden Files

For complex output (HTML, protobuf, large JSON), store the expected output in a file:

```go
func TestRenderPage(t *testing.T) {
    result := renderPage(testInput)
    golden := filepath.Join("testdata", "expected_page.html")
    if *update {
        os.WriteFile(golden, []byte(result), 0644)
    }
    expected, _ := os.ReadFile(golden)
    if result != string(expected) {
        t.Errorf("got != want. Update with -update flag")
    }
}
```

Run `go test -update` to regenerate golden files when the expected output changes.
