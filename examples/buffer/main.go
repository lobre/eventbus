package main

import (
	"fmt"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	sub, err := bus.Subscribe("signals", 1)
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	go func() {
		for e := range sub.C {
			fmt.Printf("processed %v\n", e.Payload)
			time.Sleep(40 * time.Millisecond)
		}
	}()

	for i := 0; i < 5; i++ {
		payload := fmt.Sprintf("burst-%d", i)
		_ = bus.Publish(eventbus.NewEvent("signals", "Burst", payload))
	}

	time.Sleep(300 * time.Millisecond)
	fmt.Println("buffer=1 caused older events to be dropped during bursts")
}
