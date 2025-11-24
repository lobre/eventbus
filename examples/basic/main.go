package main

import (
	"fmt"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	sub, _ := bus.Subscribe("chat", bus.Start())
	defer sub.Close()

	go func() {
		for e := range sub.C {
			fmt.Printf("received topic=%s type=%s payload=%v\n", e.Topic, e.Type, e.Payload)
		}
	}()

	last := bus.Start()
	last, _ = bus.Publish("chat", "MessagePosted", "hello", last)
	bus.Publish("chat", "MessagePosted", "world", last)

	time.Sleep(50 * time.Millisecond)
}
