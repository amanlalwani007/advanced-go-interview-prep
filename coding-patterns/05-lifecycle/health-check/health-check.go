package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type HealthStatus int

const (
	Healthy HealthStatus = iota
	Degraded
	Unhealthy
)

func (s HealthStatus) String() string {
	return [...]string{"healthy", "degraded", "unhealthy"}[s]
}

type HealthCheck struct {
	mu     sync.RWMutex
	name   string
	status HealthStatus
	check  func() error
}

func NewHealthCheck(name string, check func() error) *HealthCheck {
	return &HealthCheck{name: name, status: Healthy, check: check}
}

func (h *HealthCheck) Run() {
	err := h.check()
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		if h.status == Healthy {
			h.status = Degraded
		} else {
			h.status = Unhealthy
		}
	} else {
		h.status = Healthy
	}
}

func (h *HealthCheck) Status() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

type HealthChecker struct {
	checks []*HealthCheck
	period time.Duration
}

func NewHealthChecker(period time.Duration, checks ...*HealthCheck) *HealthChecker {
	return &HealthChecker{checks: checks, period: period}
}

func (hc *HealthChecker) Start(stop chan struct{}) {
	ticker := time.NewTicker(hc.period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var wg sync.WaitGroup
			for _, c := range hc.checks {
				wg.Add(1)
				go func(ch *HealthCheck) {
					defer wg.Done()
					ch.Run()
				}(c)
			}
			wg.Wait()
			hc.report()
		case <-stop:
			return
		}
	}
}

func (hc *HealthChecker) report() {
	for _, c := range hc.checks {
		fmt.Printf("[health] %s: %s\n", c.name, c.Status())
	}
}

func main() {
	checks := []*HealthCheck{
		NewHealthCheck("database", func() error {
			if rand.Float32() < 0.3 {
				return fmt.Errorf("connection timeout")
			}
			return nil
		}),
		NewHealthCheck("cache", func() error {
			return nil
		}),
	}

	checker := NewHealthChecker(1*time.Second, checks...)

	stop := make(chan struct{})
	go checker.Start(stop)

	time.Sleep(3 * time.Second)
	close(stop)
	fmt.Println("health check stopped")
}
