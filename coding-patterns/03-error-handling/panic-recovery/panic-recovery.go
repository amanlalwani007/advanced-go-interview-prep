package main

import (
	"fmt"
	"runtime/debug"
	"time"
)

type SafeGo struct {
	Errors chan error
}

func NewSafeGo() *SafeGo {
	return &SafeGo{Errors: make(chan error, 10)}
}

func (s *SafeGo) Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.Errors <- fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}

func main() {
	sg := NewSafeGo()

	sg.Go(func() {
		fmt.Println("task 1 running")
	})

	sg.Go(func() {
		panic("something went wrong!")
	})

	sg.Go(func() {
		fmt.Println("task 3 running")
	})

	time.Sleep(100 * time.Millisecond)
	close(sg.Errors)

	for err := range sg.Errors {
		fmt.Printf("captured: %v\n", err)
	}
}
