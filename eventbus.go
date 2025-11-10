package eventbus

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Event is the unit that gets stored and published.
type Event struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Topic     string    `json:"topic"`
	Type      string    `json:"type"`
	Payload   any       `json:"payload"`
}

// SubscribeOptions configures a subscription.
type SubscribeOptions struct {
	// Topic filters events. If empty, the subscriber receives all events.
	Topic string

	// BufferSize is the size of the internal channel.
	// If 0, a default buffer size is used.
	BufferSize int
}

// Subscription is used to receive events and stop listening.
type Subscription struct {
	// C is the channel where events are delivered.
	C <-chan Event

	// Close stops the subscription and frees resources.
	Close func()
}

// EventStore represents an append-only event log.
type EventStore interface {
	Append(e Event) (int, error)
	All() []Event
}

// InMemoryEventStore is a goroutine-safe in-memory event store.
type InMemoryEventStore struct {
	mu     sync.RWMutex
	events []Event
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events: make([]Event, 0),
	}
}

func (s *InMemoryEventStore) Append(e Event) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, e)
	return len(s.events) - 1, nil
}

func (s *InMemoryEventStore) All() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// Bus is the main pub/sub interface.
type Bus interface {
	Publish(ctx context.Context, e Event) error
	Subscribe(opts SubscribeOptions) (*Subscription, error)
	Events() EventStore
}

type subscriber struct {
	topic string
	ch    chan Event
}

type InMemoryBus struct {
	mu          sync.Mutex
	store       EventStore
	subscribers map[*subscriber]struct{}
}

// NewBus creates an InMemoryBus with an optional initial event slice.
func NewBus(initialEvents []Event) *InMemoryBus {
	store := NewInMemoryEventStore()
	for _, e := range initialEvents {
		_, _ = store.Append(e)
	}

	return &InMemoryBus{
		store:       store,
		subscribers: make(map[*subscriber]struct{}),
	}
}

func (b *InMemoryBus) Events() EventStore {
	return b.store
}

func bufferSizeOrDefault(size int) int {
	if size <= 0 {
		return 1024
	}
	return size
}

// Subscribe registers a new subscriber.
func (b *InMemoryBus) Subscribe(opts SubscribeOptions) (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &subscriber{
		topic: opts.Topic,
		ch:    make(chan Event, bufferSizeOrDefault(opts.BufferSize)),
	}

	if b.subscribers == nil {
		b.subscribers = make(map[*subscriber]struct{})
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

// Publish appends the event to the store and fans it out to subscribers.
// If a subscriber's channel is full, the event is dropped for that subscriber.
func (b *InMemoryBus) Publish(ctx context.Context, e Event) error {
	if _, err := b.store.Append(e); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for sub := range b.subscribers {
		if sub.topic == "" || sub.topic == e.Topic {
			select {
			case sub.ch <- e:
			default:
				// buffer full: drop for this subscriber
			}
		}
	}

	return nil
}

// SaveToFile writes all events in the store to a JSON file.
func SaveToFile(store EventStore, path string) error {
	events := store.All()

	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// LoadFromFile reads events from a JSON file created by SaveToFile.
func LoadFromFile(path string) ([]Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}

	return events, nil
}

