package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Bulkhead struct {
	ch chan struct{}
	wg sync.WaitGroup
}

func NewBulkhead(maxConcurrent int) *Bulkhead {
	return &Bulkhead{
		ch: make(chan struct{}, maxConcurrent),
	}
}

func (b *Bulkhead) Execute(task func()) bool {
	select {
	case b.ch <- struct{}{}:
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			defer func() { <-b.ch }()
			task()
		}()
		return true
	default:
		return false
	}
}

func (b *Bulkhead) Wait() {
	b.wg.Wait()
}

func main() {
	b := NewBulkhead(3)
	results := make(chan int, 10)

	for i := 0; i < 10; i++ {
		id := i + 1
		accepted := b.Execute(func() {
			work := time.Duration(200+rand.Intn(300)) * time.Millisecond
			time.Sleep(work)
			results <- id
		})
		if accepted {
			fmt.Printf("task %d: ✅ accepted\n", id)
		} else {
			fmt.Printf("task %d: ❌ rejected (bulkhead full)\n", id)
		}
		time.Sleep(50 * time.Millisecond)
	}

	go func() {
		b.Wait()
		close(results)
	}()

	for id := range results {
		fmt.Printf("completed: task %d\n", id)
	}
}
