package main

import (
	"fmt"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	_ = bus.Publish(eventbus.NewEvent("orders", "Placed", map[string]string{"id": "A1"}))
	_ = bus.Publish(eventbus.NewEvent("orders", "Placed", map[string]string{"id": "B2"}))

	bus.ForEachEvent("orders", applyToProjection)

	sub, err := bus.Subscribe("orders", 16)
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	go func() {
		for e := range sub.C {
			applyToProjection(e)
		}
	}()

	_ = bus.Publish(eventbus.NewEvent("orders", "Placed", map[string]string{"id": "C3"}))

	time.Sleep(50 * time.Millisecond)
}

func applyToProjection(e eventbus.Event) {
	fmt.Printf("projection observed id=%s\n", e.Payload.(map[string]string)["id"])
}
