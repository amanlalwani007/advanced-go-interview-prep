package main

import (
	"errors"
	"fmt"
)

type Result[T any] struct {
	Value T
	Err   error
}

func Ok[T any](v T) Result[T] { return Result[T]{Value: v} }
func Err[T any](e error) Result[T] { return Result[T]{Err: e} }

func (r Result[T]) IsOk() bool     { return r.Err == nil }
func (r Result[T]) IsErr() bool    { return r.Err != nil }
func (r Result[T]) Unwrap() (T, error) { return r.Value, r.Err }

func divide(a, b int) Result[int] {
	if b == 0 {
		return Err[int](errors.New("division by zero"))
	}
	return Ok(a / b)
}

func main() {
	r1 := divide(10, 2)
	if val, err := r1.Unwrap(); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println("result:", val)
	}

	r2 := divide(10, 0)
	if val, err := r2.Unwrap(); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println("result:", val)
	}
}
