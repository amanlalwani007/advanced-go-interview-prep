package main

import (
	"fmt"
)

func tee(in <-chan int) (<-chan int, <-chan int) {
	out1 := make(chan int)
	out2 := make(chan int)

	go func() {
		defer close(out1)
		defer close(out2)

		for v := range in {
			for i := 0; i < 2; i++ {
				select {
				case out1 <- v:
				case out2 <- v:
				}
			}
		}
	}()
	return out1, out2
}

func main() {
	in := make(chan int)
	go func() {
		defer close(in)
		for i := 1; i <= 5; i++ {
			in <- i
		}
	}()

	a, b := tee(in)

	count := 0
	for a != nil || b != nil {
		select {
		case v, ok := <-a:
			if ok {
				fmt.Printf("a received: %d\n", v)
			} else {
				a = nil
			}
		case v, ok := <-b:
			if ok {
				fmt.Printf("b received: %d\n", v)
			} else {
				b = nil
			}
		}
		count++
	}
}
