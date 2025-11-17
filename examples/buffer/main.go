package main

import (
	"fmt"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	sub, _ := bus.Subscribe("signals", 0, 1)
	defer sub.Close()

	go func() {
		for e := range sub.C {
			fmt.Printf("processed %v\n", e.Payload)
			time.Sleep(40 * time.Millisecond)
		}
	}()

	var last uint64
	for i := 0; i < 5; i++ {
		payload := fmt.Sprintf("burst-%d", i)
		last, _ = bus.Publish("signals", "Burst", last, payload)
	}

	time.Sleep(300 * time.Millisecond)
	fmt.Println("buffer=1 caused older events to be dropped during bursts")
}
