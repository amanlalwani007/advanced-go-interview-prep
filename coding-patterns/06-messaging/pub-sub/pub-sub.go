package main

import (
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Topic   string
	Payload interface{}
}

type PubSub struct {
	mu       sync.RWMutex
	channels map[string][]chan Event
}

func NewPubSub() *PubSub {
	return &PubSub{channels: make(map[string][]chan Event)}
}

func (ps *PubSub) Subscribe(topic string, buf int) chan Event {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ch := make(chan Event, buf)
	ps.channels[topic] = append(ps.channels[topic], ch)
	return ch
}

func (ps *PubSub) Publish(topic string, payload interface{}) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	evt := Event{Topic: topic, Payload: payload}
	for _, ch := range ps.channels[topic] {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (ps *PubSub) Unsubscribe(topic string, ch chan Event) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	subs := ps.channels[topic]
	for i, sub := range subs {
		if sub == ch {
			ps.channels[topic] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

func main() {
	ps := NewPubSub()

	sub1 := ps.Subscribe("orders", 10)
	sub2 := ps.Subscribe("orders", 10)

	go func() {
		for evt := range sub1 {
			fmt.Printf("sub1 received: %v\n", evt.Payload)
		}
	}()
	go func() {
		for evt := range sub2 {
			fmt.Printf("sub2 received: %v\n", evt.Payload)
		}
	}()

	ps.Publish("orders", "order-123: created")
	ps.Publish("orders", "order-456: created")
	ps.Publish("notifications", "should not appear")

	time.Sleep(100 * time.Millisecond)
	ps.Unsubscribe("orders", sub1)
	ps.Publish("orders", "order-789: created")
	time.Sleep(100 * time.Millisecond)
}
