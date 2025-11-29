package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	sub, err := bus.Subscribe(eventbus.AllTopics, bus.Start())
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	go routeEmails(sub.C)

	mustPublish(bus, "users", "user_registered", "casey@example.com")
	mustPublish(bus, "orders", "order_placed", "alex@example.com")
	mustPublish(bus, "users", "password_reset_requested", "riley@example.com")

	time.Sleep(100 * time.Millisecond)
}

func routeEmails(ch <-chan eventbus.Event) {
	for e := range ch {
		switch e.Type {
		case "user_registered":
			email := e.Payload.(string)
			sendEmail("Welcome", fmt.Sprintf("Hi %s, thanks for joining!", email))
			
		case "order_placed":
			email := e.Payload.(string)
			sendEmail("Order confirmation", fmt.Sprintf("Order confirmed for %s", email))
			
		case "password_reset_requested":
			email := e.Payload.(string)
			sendEmail("Password reset", fmt.Sprintf("Reset link sent to %s", email))
		}
	}
}

func sendEmail(subject, body string) {
	fmt.Printf("[email] subject=%q body=%q\n", subject, body)
}

func mustPublish(bus *eventbus.Bus, topic, eventType string, payload any) {
	_, err := bus.Publish(topic, eventType, payload, bus.End())
	if err != nil {
		log.Fatalf("publish %s: %v", eventType, err)
	}
}
