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
	seed(bus)

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		lastID := parseLastEventID(r.Header.Get("Last-Event-ID"))
		sub, _ := bus.Subscribe("notifications", lastID, eventbus.BufferDefault)
		defer sub.Close()

		setupSSEHeaders(w)
		flusher, _ := w.(http.Flusher)

		for {
			select {
			case e, ok := <-sub.C:
				if !ok {
					return
				}
				writeSSE(w, e)
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
		payload := map[string]string{
			"msg": msg,
			"ts":  time.Now().Format(time.RFC3339),
		}
		bus.Publish("notifications", "Ping", bus.LastID(), payload)
		w.WriteHeader(http.StatusAccepted)
	})

	log.Println("SSE stream on http://localhost:8080/events")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func seed(bus *eventbus.Bus) {
	var last uint64
	last, _ = bus.Publish("notifications", "Ping", last, map[string]string{"msg": "hello"})
	bus.Publish("notifications", "Ping", last, map[string]string{"msg": "world"})
}

func setupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
}

func writeSSE(w http.ResponseWriter, e eventbus.Event) {
	fmt.Fprintf(w, "id: %d\n", e.ID)
	fmt.Fprintf(w, "event: %s\n", e.Type)
	fmt.Fprintf(w, "data: %v\n\n", e.Payload)
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
