package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	// seed a few events
	_ = bus.Publish(eventbus.NewEvent("notifications", "Ping", map[string]string{"msg": "hello"}))
	_ = bus.Publish(eventbus.NewEvent("notifications", "Ping", map[string]string{"msg": "world"}))

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		lastID := parseLastEventID(r.Header.Get("Last-Event-ID"))

		sub, err := bus.Subscribe("notifications", eventbus.WithFromID(lastID))
		if err != nil {
			http.Error(w, "subscribe failed", http.StatusInternalServerError)
			return
		}
		defer sub.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		for {
			select {
			case e, ok := <-sub.C:
				if !ok {
					return
				}
				fmt.Fprintf(w, "id: %d\n", e.ID)
				fmt.Fprintf(w, "event: %s\n", e.Type)
				fmt.Fprintf(w, "data: %v\n\n", e.Payload)
				if flusher != nil {
					flusher.Flush()
				}
			case <-r.Context().Done():
				return
			}
		}
	})

	http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			msg = "tick"
		}
		_ = bus.Publish(eventbus.NewEvent("notifications", "Ping", map[string]string{"msg": msg, "ts": time.Now().Format(time.RFC3339)}))
		w.WriteHeader(http.StatusAccepted)
	})

	log.Println("SSE stream on http://localhost:8080/events")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func parseLastEventID(raw string) uint64 {
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
