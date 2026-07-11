# Messaging Patterns

Reliable async communication between components.

```
06-messaging/
├── pub-sub/            # In-memory publish-subscribe broker
└── idempotency-key/    # Deduplicate duplicate requests
```

---

## Pub/Sub

**File:** [`pub-sub/pub-sub.go`](pub-sub/pub-sub.go)

### What It Does

An in-memory event broker where publishers send events by topic and subscribers receive them on channels:

```go
ps := NewPubSub()

// Subscribe
ch := ps.Subscribe("orders", 10)

// Publish
ps.Publish("orders", "order-123: created")

// Unsubscribe
ps.Unsubscribe("orders", ch)
```

- Each topic has a list of subscriber channels.
- Publishing to a topic sends the event to all subscriber channels for that topic.
- Non-blocking send with `select { case ch <- evt: default: }` drops events if a subscriber's buffer is full.

### Key Implementation Details

- `sync.RWMutex` guards the subscriber map — concurrent publishes and subscribes are safe.
- Subscribers are stored in `map[string][]chan Event`.
- Publish iterates over all subscribers and sends with `select/default` for non-blocking delivery.
- Unsubscribe removes the channel from the slice and closes it to notify the subscriber.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| In-process event-driven communication | Cross-process messaging (use Kafka, RabbitMQ, NATS) |
| Decoupling components within the same service | Guaranteed delivery (in-memory pub/sub loses messages on crash) |
| Event sourcing in a single process | Fan-out to 1000+ subscribers (channel overhead) |

### Real-World Scenarios

**Order processing.** `OrderService.Create()` publishes `"order.created"` with the order payload. Two subscribers react:
- `EmailService` sends a confirmation email.
- `InventoryService` reserves stock.
- `AnalyticsService` records the event.
Adding a third subscriber (e.g., `FraudDetectionService`) requires zero changes to `OrderService`.

**Webhook dispatcher.** An internal event `"user.signed_up"` triggers: welcome email, Slack notification to #new-users, CRM record creation, and onboarding task creation. Each handler subscribes independently. If one handler's buffer fills up, only that subscriber drops events — others are unaffected.

### Limitations

- In-memory only — events are lost on process restart.
- Subscriber channels are unbounded or drop events if full — no backpressure.
- No persistence, no replay, no consumer groups.
- For production messaging, use a message broker (Kafka, RabbitMQ, NATS, Redis Streams).

---

## Idempotency Key

**File:** [`idempotency-key/idempotency-key.go`](idempotency-key/idempotency-key.go)

### What It Does

Guarantees that a request is processed exactly once, even if received multiple times:

```go
func (p *PaymentProcessor) Charge(idempotencyKey string, amount float64) (string, error) {
    if result, ok := p.store.Get(idempotencyKey); ok {
        return result, fmt.Errorf("duplicate request: already processed as %s", result)
    }
    txnID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
    p.store.Set(idempotencyKey, txnID)
    return txnID, nil
}
```

**Flow:**
```
Client                          Server
  │                                │
  ├── POST /charge ───────────────►│  idempotency-key: "abc-123"
  │   idempotency-key: "abc-123"  │  Process: charge card, store result
  │                                │  Return: "txn_98765"
  │◄── 200 OK, "txn_98765" ───────┤
  │                                │
  ├── POST /charge (retry) ───────►│  idempotency-key: "abc-123"
  │   idempotency-key: "abc-123"  │  Lookup: found "txn_98765"
  │                                │  Return: "txn_98765" (no charge)
  │◄── 200 OK, "txn_98765" ───────┤
```

### Key Implementation Details

- The store is a `map[string]string` with TTL-based eviction.
- `Get()` returns the stored result if the key was already processed.
- `Set()` stores the result and schedules a TTL cleanup via `time.AfterFunc`.
- The key must come from the client (UUID) — the server cannot generate it because the client's retry would have a different key.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Payment processing | Read-only operations (GET requests) |
| Order creation | Operations where duplicates are acceptable |
| Webhook handling (exactly-once delivery) | Operations with natural idempotency (e.g., "set status=X") |
| Any POST/PUT where the client might retry | Operations with server-generated idempotency (use ETags) |

### Real-World Scenarios

**Payment gateway.** `POST /charges` with `Idempotency-Key: uuid-123`. First request charges the card $99 and stores `txn_98765`. Network timeout causes the client to retry with the same key. The gateway returns `txn_98765` without charging the card again.

**Webhook receiver.** Stripe sends the same `invoice.paid` webhook twice (at-least-once delivery). Your webhook handler checks `Idempotency-Key: evt_abc123`. The second arrival returns the stored `"processed"` response. The customer is not double-billed.

**Order API.** `POST /orders` with `Idempotency-Key: order-req-456`. The request creates an order and decrements inventory. Network retry sends the same key. The idempotency store returns the existing order ID. Inventory is not double-decremented.

### Key Design Decisions

| Decision | Why |
|----------|-----|
| **Client generates the key** | Only the client knows which requests are retries. |
| **Include key in response headers** | So clients can correlate responses to keys. |
| **TTL on stored keys** | Prevents unbounded memory growth. Typical TTL: 24h. |
| **Store both success and failure** | A retry after a failure should retry, not return the cached failure. |
