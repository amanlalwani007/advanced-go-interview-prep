package main

import (
	"fmt"
	"time"
)

func or(chs ...<-chan struct{}) <-chan struct{} {
	switch len(chs) {
	case 0:
		c := make(chan struct{})
		close(c)
		return c
	case 1:
		return chs[0]
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-chs[0]:
		case <-chs[1]:
		case <-or(chs[2:]...):
		}
	}()
	return done
}

func main() {
	a := time.After(3 * time.Second)
	b := time.After(2 * time.Second)
	c := time.After(5 * time.Second)

	start := time.Now()
	<-or(
		toSignal(a),
		toSignal(b),
		toSignal(c),
	)
	fmt.Printf("done after %v (fastest was 2s)\n", time.Since(start).Round(time.Second))
}

func toSignal(ch <-chan time.Time) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-ch
		close(done)
	}()
	return done
}
