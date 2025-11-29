package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	// Car dashboard subscribes with buffer size 1: many low-fuel signals collapse into "latest only".
	sub, err := bus.SubscribeWithBufferSize("fuel", bus.Start(), 1)
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	go runFuelSensor(bus)

	updateDashboard(sub.C, time.After(120*time.Millisecond))
	time.Sleep(40 * time.Millisecond)
}

func runFuelSensor(bus *eventbus.Bus) {
	for _, liters := range []float64{7.5, 7.2, 7.0, 6.8, 6.5, 6.2, 6.1, 6.0, 5.9, 5.7, 5.5, 5.4} {
		_, err := bus.Publish("fuel", "low_fuel", liters, bus.End())
		if err != nil {
			log.Fatalf("publish fuel signal: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func updateDashboard(ch <-chan eventbus.Event, ignitionOff <-chan time.Time) {
	for {
		select {
		case e := <-ch:
			fmt.Printf("Dashboard low-fuel indicator updated: %.1f L remaining\n", e.Payload.(float64))
			time.Sleep(100 * time.Millisecond)
		case <-ignitionOff:
			fmt.Println("Ignition off: dashboard stops refreshing")
			return
		}
	}
}
