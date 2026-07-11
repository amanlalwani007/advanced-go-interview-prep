package main

import (
	"fmt"
	"sync"
	"time"
)

type TokenBucket struct {
	mu        sync.Mutex
	capacity  int
	tokens    int
	rate      float64
	lastRefill time.Time
}

func NewTokenBucket(capacity int, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = int(float64(tb.capacity))
	added := int(elapsed * tb.rate)
	if tb.tokens+added < tb.capacity {
		tb.tokens += added
	} else {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

func main() {
	tb := NewTokenBucket(5, 2)
	for i := 0; i < 10; i++ {
		if tb.Allow() {
			fmt.Printf("request %d: ✅ allowed\n", i+1)
		} else {
			fmt.Printf("request %d: ❌ rate limited\n", i+1)
		}
		time.Sleep(300 * time.Millisecond)
	}
}
