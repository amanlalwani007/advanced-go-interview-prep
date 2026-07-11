package main

import (
	"fmt"
	"sync"
	"time"
)

type IdempotencyStore struct {
	mu    sync.Mutex
	store map[string]string
	ttl   time.Duration
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{store: make(map[string]string), ttl: ttl}
}

func (is *IdempotencyStore) Get(key string) (string, bool) {
	is.mu.Lock()
	defer is.mu.Unlock()
	v, ok := is.store[key]
	return v, ok
}

func (is *IdempotencyStore) Set(key, value string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.store[key] = value
	time.AfterFunc(is.ttl, func() {
		is.mu.Lock()
		delete(is.store, key)
		is.mu.Unlock()
	})
}

type PaymentProcessor struct {
	store *IdempotencyStore
}

func (p *PaymentProcessor) Charge(idempotencyKey string, amount float64) (string, error) {
	if result, ok := p.store.Get(idempotencyKey); ok {
		return result, fmt.Errorf("duplicate request: already processed as %s", result)
	}
	txnID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	p.store.Set(idempotencyKey, txnID)
	return txnID, nil
}

func main() {
	pp := &PaymentProcessor{store: NewIdempotencyStore(1 * time.Minute)}

	key := "idem-001"
	r1, err1 := pp.Charge(key, 100.0)
	fmt.Printf("first: id=%s err=%v\n", r1, err1)

	r2, err2 := pp.Charge(key, 100.0)
	fmt.Printf("second: id=%s err=%v\n", r2, err2)
}
