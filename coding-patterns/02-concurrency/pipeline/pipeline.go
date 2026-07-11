package main

import (
	"fmt"
	"strings"
	"time"
)

type Stage func(<-chan string) <-chan string

func Pipeline(stages ...Stage) Stage {
	return func(in <-chan string) <-chan string {
		for _, stage := range stages {
			in = stage(in)
		}
		return in
	}
}

func lowercase() Stage {
	return func(in <-chan string) <-chan string {
		out := make(chan string)
		go func() {
			defer close(out)
			for s := range in {
				time.Sleep(10 * time.Millisecond)
				out <- strings.ToLower(s)
			}
		}()
		return out
	}
}

func splitWords() Stage {
	return func(in <-chan string) <-chan string {
		out := make(chan string)
		go func() {
			defer close(out)
			for s := range in {
				for _, w := range strings.Fields(s) {
					out <- w
				}
			}
		}()
		return out
	}
}

func filterLongWords() Stage {
	return func(in <-chan string) <-chan string {
		out := make(chan string)
		go func() {
			defer close(out)
			for w := range in {
				if len(w) <= 4 {
					out <- w
				}
			}
		}()
		return out
	}
}

func main() {
	sentences := []string{
		"Hello World Example",
		"Go Pipelines Are Powerful",
		"Short words only",
	}

	in := make(chan string)
	go func() {
		for _, s := range sentences {
			in <- s
		}
		close(in)
	}()

	p := Pipeline(lowercase(), splitWords(), filterLongWords())
	for w := range p(in) {
		fmt.Println(w)
	}
}
