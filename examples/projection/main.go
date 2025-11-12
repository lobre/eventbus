package main

import (
	"fmt"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	_ = bus.Publish(eventbus.NewEvent("payments", "Received", map[string]int{"amount": 5}))
	_ = bus.Publish(eventbus.NewEvent("payments", "Received", map[string]int{"amount": 7}))

	cp := checkpoint{}
	cp = replayPayments(bus, cp)
	fmt.Printf("initial total=%d processed=%d\n", cp.amount, cp.processed)

	_ = bus.Publish(eventbus.NewEvent("payments", "Received", map[string]int{"amount": 10}))

	cp = replayPayments(bus, cp)
	fmt.Printf("after catch-up total=%d processed=%d\n", cp.amount, cp.processed)
}

type checkpoint struct {
	amount    int
	processed int
}

func replayPayments(bus *eventbus.Bus, from checkpoint) checkpoint {
	result := from
	seen := 0

	bus.ForEachEvent("payments", func(e eventbus.Event) {
		seen++
		if seen <= from.processed {
			return
		}
		payload := e.Payload.(map[string]int)
		result.amount += payload["amount"]
		result.processed = seen
	})

	return result
}
