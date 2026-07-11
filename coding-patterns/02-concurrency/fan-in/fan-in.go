package main

import (
	"fmt"
	"sync"
	"time"
)

func fanIn(chs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	wg.Add(len(chs))

	for _, ch := range chs {
		go func(c <-chan int) {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func source(name string, count int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; i < count; i++ {
			time.Sleep(30 * time.Millisecond)
			ch <- i
		}
	}()
	return ch
}

func main() {
	out := fanIn(
		source("A", 3),
		source("B", 3),
		source("C", 3),
	)

	for v := range out {
		fmt.Printf("merged: %d\n", v)
	}
}
