package main

import "fmt"

func Add(a, b int) int {
	return a + b
}

func Divide(a, b int) (int, error) {
	if b == 0 {
		return 0, ErrDivisionByZero
	}
	return a / b, nil
}

var ErrDivisionByZero = fmt.Errorf("division by zero")

func main() {
}
