package eventbus

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Event is the unit that gets stored and published.
type Event struct {
	ID        uint64    `json:"id"`
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

const defaultBufferSize = 1024

type subscribeConfig struct {
	bufferSize int
	fromID     *uint64
}

// SubscribeOption configures Subscribe behavior.
type SubscribeOption func(*subscribeConfig)

// WithBufferSize sets the channel buffer size (defaults to 1024).
// When the buffer is full, new events for that subscriber are dropped.
func WithBufferSize(size int) SubscribeOption {
	return func(cfg *subscribeConfig) {
		if size <= 0 {
			cfg.bufferSize = defaultBufferSize
		} else {
			cfg.bufferSize = size
		}
	}
}

// WithFromID replays events with ID greater than id before delivering live ones.
func WithFromID(id uint64) SubscribeOption {
	return func(cfg *subscribeConfig) {
		cfg.fromID = &id
	}
}

// Bus is an in-memory pub/sub bus with an append-only event log.
type Bus struct {
	mu          sync.Mutex
	events      []Event
	subscribers map[*subscriber]struct{}
	nextID      uint64
}

// New creates a Bus with an empty event log.
func New() *Bus {
	return &Bus{
		events:      make([]Event, 0),
		subscribers: make(map[*subscriber]struct{}),
		nextID:      0,
	}
}

// ForEachEvent calls fn for every event in the log.
// If topic is non-empty, only events with that topic are processed.
func (b *Bus) ForEachEvent(topic string, fn func(Event)) {
	b.mu.Lock()
	events := append([]Event(nil), b.events...)
	b.mu.Unlock()

	for _, e := range events {
		if topic == "" || e.Topic == topic {
			fn(e)
		}
	}
}

// Dump writes all events as JSON to w.
// It does not affect subscribers.
func (b *Bus) Dump(w io.Writer) error {
	b.mu.Lock()
	events := append([]Event(nil), b.events...)
	b.mu.Unlock()

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

	b.mu.Lock()
	b.events = append([]Event(nil), events...)
	if len(b.events) == 0 {
		b.nextID = 0
	} else {
		b.nextID = b.events[len(b.events)-1].ID + 1
	}
	b.mu.Unlock()

	return nil
}

// Subscribe registers a new subscriber for a topic.
// If topic is empty, the subscriber receives all events.
func (b *Bus) Subscribe(topic string, opts ...SubscribeOption) (*Subscription, error) {
	cfg := subscribeConfig{
		bufferSize: defaultBufferSize,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	sub := &subscriber{
		topic: topic,
		ch:    make(chan Event, cfg.bufferSize),
	}

	var history []Event

	b.mu.Lock()
	if cfg.fromID != nil {
		history = make([]Event, 0, len(b.events))
		for _, e := range b.events {
			if (topic == "" || e.Topic == topic) && e.ID > *cfg.fromID {
				history = append(history, e)
			}
		}
	}
	b.subscribers[sub] = struct{}{}
	b.mu.Unlock()

	subscription := &Subscription{
		C: sub.ch,
		Close: func() {
			var ch chan Event
			b.mu.Lock()
			if _, ok := b.subscribers[sub]; ok {
				delete(b.subscribers, sub)
				ch = sub.ch
			}
			b.mu.Unlock()

			if ch != nil {
				close(ch)
			}
		},
	}

	if cfg.fromID != nil && len(history) > 0 {
		go func(events []Event, ch chan Event) {
			for _, e := range events {
				select {
				case ch <- e:
				default:
					// buffer full: drop replayed event for this subscriber
				}
			}
		}(history, sub.ch)
	}

	return subscription, nil
}

// Publish appends the event to the log and fans it out to subscribers.
// If a subscriber's channel is full, the event is dropped for that subscriber.
func (b *Bus) Publish(e Event) error {
	b.mu.Lock()
	e.ID = b.nextID
	b.nextID++

	b.events = append(b.events, e)

	for sub := range b.subscribers {
		if sub.topic != "" && sub.topic != e.Topic {
			continue
		}

		select {
		case sub.ch <- e:
		default:
			// buffer full: drop for this subscriber
		}
	}

	b.mu.Unlock()

	return nil
}

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
