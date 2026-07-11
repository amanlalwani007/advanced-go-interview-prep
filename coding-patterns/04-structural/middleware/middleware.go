package main

import (
	"fmt"
	"log"
	"time"
)

type Handler func(string) string

type Middleware func(Handler) Handler

func Chain(h Handler, middlewares ...Middleware) Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func Logging(next Handler) Handler {
	return func(s string) string {
		start := time.Now()
		result := next(s)
		log.Printf("request=%q result=%q duration=%v", s, result, time.Since(start))
		return result
	}
}

func Recovery(next Handler) Handler {
	return func(s string) (result string) {
		defer func() {
			if r := recover(); r != nil {
				result = fmt.Sprintf("recovered: %v", r)
			}
		}()
		return next(s)
	}
}

func main() {
	hello := func(s string) string {
		return "hello " + s
	}

	h := Chain(hello, Logging, Recovery)
	fmt.Println(h("world"))
}
