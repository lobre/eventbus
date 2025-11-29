package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	path := "todo.json"
	defer os.Remove(path)

	bus := eventbus.New()

	addTodo(bus, "Write README")
	addTodo(bus, "Ship demo")

	if err := bus.SaveToFile(path); err != nil {
		log.Fatalf("save log: %v", err)
	}
	fmt.Println("Saved log to", path)

	time.Sleep(2 * time.Second)

	reloaded, err := eventbus.NewFromFile(path)
	if err != nil {
		log.Fatalf("reload log: %v", err)
	}

	fmt.Println("Replaying tasks from file:")
	reloaded.ForEachEvent(eventbus.Query{Topic: "todo"}, func(e eventbus.Event) {
		text, ok := e.Payload.(string)
		if !ok {
			log.Printf("unexpected payload: %#v", e.Payload)
			return
		}
		fmt.Printf("- %s\n", text)
	})
}

func addTodo(bus *eventbus.Bus, text string) {
	_, err := bus.Publish("todo", "task_created", text, bus.End())
	if err != nil {
		log.Fatalf("publish todo: %v", err)
	}
}
