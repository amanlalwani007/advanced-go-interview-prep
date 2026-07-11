package main

import (
	"context"
	"fmt"
	"time"
)

func slowOperation(ctx context.Context) error {
	select {
	case <-time.After(3 * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := slowOperation(ctx)
	fmt.Printf("elapsed: %v\n", time.Since(start))
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		fmt.Println("success")
	}
}
