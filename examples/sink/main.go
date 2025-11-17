package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	path := "events.log"
	defer os.Remove(path)

	bus := eventbus.New()

	sink, _ := bus.Subscribe("", 0, eventbus.BufferDefault)
	defer sink.Close()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	go func() {
		for e := range sink.C {
			line := fmt.Sprintf("%s %s %v\n", e.Topic, e.Type, e.Payload)
			if _, err := f.WriteString(line); err != nil {
				panic(err)
			}
		}
	}()

	var last uint64
	last, _ = bus.Publish("orders", "Placed", last, map[string]string{"id": "X1"})
	last, _ = bus.Publish("orders", "Placed", last, map[string]string{"id": "X2"})
	bus.Publish("payments", "Accepted", last, map[string]string{"id": "P1"})

	time.Sleep(50 * time.Millisecond)

	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	fmt.Printf("append-only log contents:\n%s", data)
}
