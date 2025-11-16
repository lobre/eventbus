package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()
	ledger := &orderLedger{}

	sub, err := bus.Subscribe("orders")
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	go ledger.consume(sub.C)

	_ = bus.Publish(eventbus.NewEvent("orders", "Placed", map[string]string{"id": "A1", "user": "alice"}))
	_ = bus.Publish(eventbus.NewEvent("orders", "Placed", map[string]string{"id": "B2", "user": "bob"}))
	_ = bus.Publish(eventbus.NewEvent("orders", "Placed", map[string]string{"id": "C3", "user": "alice"}))

	time.Sleep(50 * time.Millisecond)

	fmt.Printf("orders per user: %v\n", ledger.snapshot())
}

type orderLedger struct {
	mu     sync.Mutex
	counts map[string]int
}

func (l *orderLedger) consume(ch <-chan eventbus.Event) {
	for e := range ch {
		payload := e.Payload.(map[string]string)
		l.mu.Lock()
		if l.counts == nil {
			l.counts = make(map[string]int)
		}
		l.counts[payload["user"]]++
		l.mu.Unlock()
	}
}

func (l *orderLedger) snapshot() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	copy := make(map[string]int, len(l.counts))
	for k, v := range l.counts {
		copy[k] = v
	}
	return copy
}
