package main

import (
	"fmt"
	"os"

	"github.com/lobre/eventbus"
)

func main() {
	path := "snapshot.json"
	defer os.Remove(path)

	bus := eventbus.New()
	bus.Publish("inventory", "Added", map[string]int{"qty": 3}, bus.Start())

	if err := bus.SaveToFile(path); err != nil {
		panic(err)
	}

	restored, err := eventbus.NewFromFile(path)
	if err != nil {
		panic(err)
	}

	restored.ForEachEvent(eventbus.Query{Topic: "inventory"}, func(e eventbus.Event) {
		fmt.Printf("restored payload=%v\n", e.Payload)
	})
}
