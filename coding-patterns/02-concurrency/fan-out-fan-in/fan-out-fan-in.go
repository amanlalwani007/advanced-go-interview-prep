package main

import (
	"fmt"
	"sync"
	"time"
)

func generate(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		for _, n := range nums {
			out <- n
		}
		close(out)
	}()
	return out
}

func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		for n := range in {
			time.Sleep(50 * time.Millisecond)
			out <- n * n
		}
		close(out)
	}()
	return out
}

func fanIn(chs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	for _, ch := range chs {
		wg.Add(1)
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

func main() {
	in := generate(1, 2, 3, 4, 5, 6)

	c1 := square(in)
	c2 := square(in)

	out := fanIn(c1, c2)
	for v := range out {
		fmt.Println(v)
	}
}
