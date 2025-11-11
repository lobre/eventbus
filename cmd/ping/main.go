package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	// Create a bus, optionally loading events from disk if the file exists.
	bus, err := eventbus.NewFromFile("events.json")
	if err != nil {
		log.Printf("failed to load events.json: %v", err)
		bus = eventbus.New()
	} else {
		log.Printf("bus initialized from events.json")
	}

	// Single subscriber that logs all events.
	startLoggingSubscriber(bus)

	// /ping publishes a PingReceived event.
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		ev := eventbus.NewEvent("ping", "PingReceived", map[string]any{
			"path":   r.URL.Path,
			"method": r.Method,
		})

		if err := bus.Publish(r.Context(), ev); err != nil {
			log.Printf("failed to publish ping event: %v", err)
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ping event published\n"))
	})

	srv := &http.Server{
		Addr: ":8080",
	}

	// Start HTTP server.
	go func() {
		log.Printf("HTTP server listening on http://localhost:8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Graceful shutdown on Ctrl+C / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Printf("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Save all events to file.
	if err := bus.SaveToFile("events.json"); err != nil {
		log.Printf("failed to save events: %v", err)
	} else {
		log.Printf("events saved to events.json")
	}

	log.Printf("bye")
}

// startLoggingSubscriber subscribes to all events and logs them.
func startLoggingSubscriber(bus *eventbus.Bus) {
	sub, err := bus.Subscribe("", 256) // all topics, arbitrary buffer size
	if err != nil {
		log.Printf("failed to subscribe logger: %v", err)
		return
	}

	go func() {
		for e := range sub.C {
			log.Printf("[EVENT] topic=%s type=%s payload=%v",
				e.Topic, e.Type, e.Payload)
		}
	}()
}

