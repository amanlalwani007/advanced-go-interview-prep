package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	name string
}

func (s *Server) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Printf("%s started\n", s.name)
	<-ctx.Done()
	fmt.Printf("%s shutting down...\n", s.name)
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("%s stopped\n", s.name)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	servers := []*Server{
		{name: "api"},
		{name: "worker"},
		{name: "monitor"},
	}

	for _, s := range servers {
		wg.Add(1)
		go s.Start(ctx, &wg)
	}

	select {
	case <-sig:
		fmt.Println("\nsignal received, shutting down...")
		cancel()
	case <-ctx.Done():
	}

	wg.Wait()
	fmt.Println("all servers stopped")
}
