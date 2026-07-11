package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

type LifecycleManager struct {
	mu       sync.Mutex
	services map[string]Service
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewLifecycleManager() *LifecycleManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &LifecycleManager{
		services: make(map[string]Service),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (lm *LifecycleManager) Register(s Service) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.services[s.Name()] = s
}

func (lm *LifecycleManager) StartAll() error {
	for name, s := range lm.services {
		if err := s.Start(lm.ctx); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fmt.Printf("started: %s\n", name)
	}
	return nil
}

func (lm *LifecycleManager) Shutdown(ctx context.Context) error {
	lm.cancel()
	var wg sync.WaitGroup
	errs := make(chan error, len(lm.services))

	for name, s := range lm.services {
		wg.Add(1)
		go func(n string, svc Service) {
			defer wg.Done()
			if err := svc.Stop(ctx); err != nil {
				errs <- fmt.Errorf("%s: %w", n, err)
			}
			fmt.Printf("stopped: %s\n", n)
		}(name, s)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

type simpleService struct {
	name   string
	start  func(context.Context) error
	stop   func(context.Context) error
}

func (s *simpleService) Start(ctx context.Context) error { return s.start(ctx) }
func (s *simpleService) Stop(ctx context.Context) error  { return s.stop(ctx) }
func (s *simpleService) Name() string                    { return s.name }

func main() {
	lm := NewLifecycleManager()

	lm.Register(&simpleService{
		name: "http-server",
		start: func(ctx context.Context) error {
			fmt.Println("http: listening on :8080")
			return nil
		},
		stop: func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	})

	lm.Register(&simpleService{
		name: "message-consumer",
		start: func(ctx context.Context) error {
			fmt.Println("consumer: connected to queue")
			return nil
		},
		stop: func(ctx context.Context) error {
			time.Sleep(300 * time.Millisecond)
			return nil
		},
	})

	lm.StartAll()
	fmt.Println("\n--- shutting down ---")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	lm.Shutdown(ctx)
	fmt.Println("done")
}
