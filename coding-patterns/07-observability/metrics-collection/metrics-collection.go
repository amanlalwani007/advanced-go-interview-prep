package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type Counter struct {
	name  string
	value atomic.Int64
}

func (c *Counter) Inc()    { c.value.Add(1) }
func (c *Counter) Value() int64 { return c.value.Load() }

type Gauge struct {
	name  string
	value atomic.Int64
}

func (g *Gauge) Set(v int64) { g.value.Store(v) }
func (g *Gauge) Value() int64 { return g.value.Load() }

type Histogram struct {
	name    string
	buckets []int64
	counts  []atomic.Int64
	total   atomic.Int64
	mu      sync.Mutex
}

func NewHistogram(name string, buckets []int64) *Histogram {
	return &Histogram{name: name, buckets: buckets, counts: make([]atomic.Int64, len(buckets))}
}

func (h *Histogram) Observe(v int64) {
	h.total.Add(1)
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i].Add(1)
			break
		}
	}
}

func (h *Histogram) Snapshot() map[string]int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	snap := map[string]int64{"total": h.total.Load()}
	for i, b := range h.buckets {
		snap[fmt.Sprintf("le_%d", b)] = h.counts[i].Load()
	}
	return snap
}

type MetricsRegistry struct {
	mu        sync.RWMutex
	counters  map[string]*Counter
	gauges    map[string]*Gauge
	histograms map[string]*Histogram
}

func NewRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

func (r *MetricsRegistry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{name: name}
	r.counters[name] = c
	return c
}

func (r *MetricsRegistry) Gauge(name string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{name: name}
	r.gauges[name] = g
	return g
}

func (r *MetricsRegistry) Report() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.counters {
		fmt.Printf("counter %s = %d\n", c.name, c.Value())
	}
	for _, g := range r.gauges {
		fmt.Printf("gauge %s = %d\n", g.name, g.Value())
	}
	for _, h := range r.histograms {
		fmt.Printf("histogram %s: %v\n", h.name, h.Snapshot())
	}
}

func main() {
	reg := NewRegistry()
	requests := reg.Counter("http_requests_total")
	conns := reg.Gauge("active_connections")

	latency := NewHistogram("request_duration_ms", []int64{5, 10, 25, 50, 100, 500})

	go func() {
		for i := 0; i < 20; i++ {
			requests.Inc()
			conns.Set(int64(rand.Intn(10)))
			latency.Observe(int64(rand.Intn(200)))
			time.Sleep(50 * time.Millisecond)
		}
	}()

	time.Sleep(1 * time.Second)
	reg.Report()
}
