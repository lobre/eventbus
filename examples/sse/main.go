package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lobre/eventbus"
)

func main() {
	bus := eventbus.New()

	last := bus.Start()
	last, _ = bus.Publish("notifications", "Ping", "hello", last)
	bus.Publish("notifications", "Ping", "world", last)

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		lastID := r.Header.Get("Last-Event-ID")
		sub, _ := bus.Subscribe("notifications", lastID)
		defer sub.Close()

		sse, _ := newSSE(w)
		for {
			select {
			case e := <-sub.C:
				sse.Write(e)
			case <-r.Context().Done():
				return
			}
		}
	})

	http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		bus.Publish("notifications", "Ping", "tick", bus.End())
		w.WriteHeader(http.StatusAccepted)
	})

	log.Println("SSE stream on http://localhost:8080/events")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type sseWriter struct {
	w     http.ResponseWriter
	flush func()
}

func newSSE(w http.ResponseWriter) (*sseWriter, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}
	return &sseWriter{w: w, flush: flusher.Flush}, nil
}

func (s *sseWriter) Write(e eventbus.Event) {
	fmt.Fprintf(s.w, "id: %s\n", e.ID)
	fmt.Fprintf(s.w, "event: %s\n", e.Type)
	fmt.Fprintf(s.w, "data: %v\n\n", e.Payload)
	s.flush()
}
