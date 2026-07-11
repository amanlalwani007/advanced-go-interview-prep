package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

func exponentialBackoff(attempt int, base time.Duration, max time.Duration) time.Duration {
	d := float64(base) * math.Pow(2, float64(attempt))
	jitter := rand.Float64() * float64(base)
	d = math.Min(d+jitter, float64(max))
	return time.Duration(d)
}

func Retry(attempts int, base time.Duration, max time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		wait := exponentialBackoff(i, base, max)
		fmt.Printf("attempt %d failed, retrying in %v...\n", i+1, wait)
		time.Sleep(wait)
	}
	return fmt.Errorf("all %d attempts failed: %w", attempts, err)
}

var counter int

func main() {
	flaky := func() error {
		counter++
		if counter < 4 {
			return fmt.Errorf("transient error #%d", counter)
		}
		return nil
	}

	err := Retry(5, 100*time.Millisecond, 2*time.Second, flaky)
	if err != nil {
		fmt.Printf("final: %v\n", err)
	} else {
		fmt.Println("succeeded after", counter, "attempts")
	}
}
