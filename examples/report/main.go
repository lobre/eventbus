package main

import (
	"fmt"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	seed(bus)

	total := 0
	bus.ForEachEvent("expenses", func(e eventbus.Event) {
		payload := e.Payload.(map[string]any)
		if payload["category"] == "food" {
			total += payload["amount"].(int)
		}
	})

	fmt.Printf("total spent on food: %d\n", total)
}

func seed(bus *eventbus.Bus) {
	_ = bus.Publish(eventbus.NewEvent("expenses", "Recorded", map[string]any{"category": "food", "amount": 25}))
	_ = bus.Publish(eventbus.NewEvent("expenses", "Recorded", map[string]any{"category": "rent", "amount": 500}))
	_ = bus.Publish(eventbus.NewEvent("expenses", "Recorded", map[string]any{"category": "food", "amount": 30}))
}
