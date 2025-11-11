package eventbus

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Event is the unit that gets stored and published.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Topic     string    `json:"topic"`
	Type      string    `json:"type"`
	Payload   any       `json:"payload"`
}

// NewEvent builds an Event with the current UTC timestamp.
func NewEvent(topic, typ string, payload any) Event {
	return Event{
		Timestamp: time.Now().UTC(),
		Topic:     topic,
		Type:      typ,
		Payload:   payload,
	}
}

// Subscription is used to receive events and stop listening.
type Subscription struct {
	C     <-chan Event
	Close func()
}

type subscriber struct {
	topic string
	ch    chan Event
}

// Bus is an in-memory pub/sub bus with an append-only event log.
type Bus struct {
	mu          sync.Mutex
	events      []Event
	subscribers map[*subscriber]struct{}
}

// New creates a Bus with an empty event log.
func New() *Bus {
	return &Bus{
		events:      make([]Event, 0),
		subscribers: make(map[*subscriber]struct{}),
	}
}

// ForEachEvent calls fn for every event in the log.
// If topic is non-empty, only events with that topic are processed.
func (b *Bus) ForEachEvent(topic string, fn func(Event)) {
	b.mu.Lock()
	eventsCopy := make([]Event, len(b.events))
	copy(eventsCopy, b.events)
	b.mu.Unlock()

	for _, e := range eventsCopy {
		if topic == "" || e.Topic == topic {
			fn(e)
		}
	}
}

// Dump writes all events as JSON to w.
// It does not affect subscribers.
func (b *Bus) Dump(w io.Writer) error {
	var events []Event
	b.ForEachEvent("", func(e Event) {
		events = append(events, e)
	})

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(events)
}

// Load reads events as JSON from r and replaces the current log.
// It does not notify subscribers.
func (b *Bus) Load(r io.Reader) error {
	var events []Event
	if err := json.NewDecoder(r).Decode(&events); err != nil {
		return err
	}

	eventsCopy := make([]Event, len(events))
	copy(eventsCopy, events)

	b.mu.Lock()
	b.events = eventsCopy
	b.mu.Unlock()

	return nil
}

// Subscribe registers a new subscriber for a topic.
// If topic is empty, the subscriber receives all events.
// If the channel buffer is full, events are dropped for that subscriber.
func (b *Bus) Subscribe(topic string, bufferSize int) (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if bufferSize <= 0 {
		bufferSize = 1024
	}

	sub := &subscriber{
		topic: topic,
		ch:    make(chan Event, bufferSize),
	}

	b.subscribers[sub] = struct{}{}

	subscription := &Subscription{
		C: sub.ch,
		Close: func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if _, ok := b.subscribers[sub]; ok {
				delete(b.subscribers, sub)
				close(sub.ch)
			}
		},
	}

	return subscription, nil
}

// Publish appends the event to the log and fans it out to subscribers.
// If a subscriber's channel is full, the event is dropped for that subscriber.
func (b *Bus) Publish(ctx context.Context, e Event) error {
	b.mu.Lock()
	b.events = append(b.events, e)

	for sub := range b.subscribers {
		if sub.topic == "" || sub.topic == e.Topic {
			select {
			case sub.ch <- e:
			default:
				// buffer full: drop for this subscriber
			}
		}
	}

	b.mu.Unlock()
	return nil
}

// Helper functions for file-based usage (optional, but handy).

// NewFromFile creates a new Bus and loads events from the given JSON file.
// If the file does not exist, it returns an empty bus and nil error.
func NewFromFile(path string) (*Bus, error) {
	b := New()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return nil, err
	}
	defer f.Close()

	if err := b.Load(f); err != nil {
		return nil, err
	}
	return b, nil
}

// SaveToFile dumps all events to the given path, overwriting the file if it exists.
func (b *Bus) SaveToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return b.Dump(f)
}
