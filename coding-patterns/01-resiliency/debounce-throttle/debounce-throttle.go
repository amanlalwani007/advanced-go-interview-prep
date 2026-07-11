package main

import (
	"fmt"
	"sync"
	"time"
)

func Debounce(fn func(), d time.Duration) func() {
	var mu sync.Mutex
	var timer *time.Timer

	return func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(d, fn)
	}
}

func Throttle(fn func(), d time.Duration) func() {
	var mu sync.Mutex
	var last time.Time

	return func() {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(last) < d {
			fmt.Print(".")
			return
		}
		last = time.Now()
		fn()
	}
}

func main() {
	print := func() { fmt.Println("fire") }

	debounced := Debounce(print, 200*time.Millisecond)
	throttled := Throttle(print, 300*time.Millisecond)

	fmt.Println("=== Debounce (last call wins) ===")
	for i := 0; i < 5; i++ {
		debounced()
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	fmt.Println("\n=== Throttle (at most once per interval) ===")
	for i := 0; i < 10; i++ {
		throttled()
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println()
}
