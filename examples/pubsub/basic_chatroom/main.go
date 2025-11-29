package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lobre/eventbus"
)

type message struct {
	From string
	Text string
}

func main() {
	bus := eventbus.New()

	sub, err := bus.Subscribe("sports", bus.Start())
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	go readMessages("alex", sub.C)

	postMessage(bus, message{From: "coach", Text: "Welcome to #sports."})
	postMessage(bus, message{From: "coach", Text: "Practice at 6pm. Bring water."})

	time.Sleep(50 * time.Millisecond)
}

func postMessage(bus *eventbus.Bus, payload message) {
	_, err := bus.Publish("sports", "MessagePosted", payload, bus.End())
	if err != nil {
		log.Fatalf("publish sports: %v", err)
	}
}

func readMessages(user string, ch <-chan eventbus.Event) {
	for e := range ch {
		msg, ok := e.Payload.(message)
		if !ok {
			log.Printf("[%s] ignoring payload of unexpected type: %#v", user, e.Payload)
			continue
		}
		fmt.Printf("[%s] %s: %s\n", user, msg.From, msg.Text)
	}
}
