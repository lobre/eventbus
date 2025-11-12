package main

import (
	"fmt"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	sub, err := bus.Subscribe("chat", 8)
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	go func() {
		for e := range sub.C {
			fmt.Printf("received topic=%s type=%s payload=%v\n", e.Topic, e.Type, e.Payload)
		}
	}()

	_ = bus.Publish(eventbus.NewEvent("chat", "MessagePosted", "hello"))
	_ = bus.Publish(eventbus.NewEvent("chat", "MessagePosted", "world"))

	time.Sleep(50 * time.Millisecond)
}
