package main

import (
	"fmt"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	sub, _ := bus.Subscribe("chat", bus.Start(), eventbus.DefaultCap)
	defer sub.Close()

	go func() {
		for e := range sub.C {
			fmt.Printf("received topic=%s type=%s payload=%v\n", e.Topic, e.Type, e.Payload)
		}
	}()

	last := bus.Start()
	last, _ = bus.Publish("chat", "MessagePosted", last, "hello")
	bus.Publish("chat", "MessagePosted", last, "world")

	time.Sleep(50 * time.Millisecond)
}
