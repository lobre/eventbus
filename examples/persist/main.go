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
	bus.Publish("inventory", "Added", bus.End(), map[string]int{"qty": 3})

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
