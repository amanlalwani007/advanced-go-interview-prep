package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

type CircuitBreaker struct {
	mu               sync.Mutex
	state            State
	failureCount     int
	successCount     int
	threshold        int
	halfOpenMax      int
	recoveryTime     time.Duration
	lastFailureTime  time.Time
}

func NewCircuitBreaker(threshold, halfOpenMax int, recoveryTime time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       Closed,
		threshold:   threshold,
		halfOpenMax: halfOpenMax,
		recoveryTime: recoveryTime,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if cb.state == Open {
		if time.Since(cb.lastFailureTime) > cb.recoveryTime {
			cb.state = HalfOpen
			cb.successCount = 0
			fmt.Println("→ Half-Open: allowing probe request")
		} else {
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker: open (fast fail)")
		}
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		fmt.Printf("✗ failure (state=%v, count=%d)\n", cb.state, cb.failureCount+1)
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		if cb.state == HalfOpen || cb.failureCount >= cb.threshold {
			cb.state = Open
			fmt.Printf("→ Open: threshold reached\n")
		}
		return err
	}

	cb.failureCount = 0
	if cb.state == HalfOpen {
		cb.successCount++
		if cb.successCount >= cb.halfOpenMax {
			cb.state = Closed
			fmt.Printf("→ Closed: recovered after %d successes\n", cb.successCount)
		}
	}
	return nil
}

func main() {
	cb := NewCircuitBreaker(3, 2, 1*time.Second)

	flaky := func() error {
		if rand.Float32() < 0.6 {
			return fmt.Errorf("service error")
		}
		return nil
	}

	for i := 0; i < 12; i++ {
		err := cb.Call(flaky)
		if err != nil {
			fmt.Printf("request %d: %v\n", i+1, err)
		} else {
			fmt.Printf("request %d: OK\n", i+1)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
