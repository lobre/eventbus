package eventbus

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
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

// Subscription exposes an events channel plus a Close function to stop delivery.
type Subscription struct {
	C     <-chan Event
	Close func()
}

type subscriber struct {
	topic string
	ch    chan Event
}

const (
	// DefaultCap configures the recommended subscriber channel capacity.
	DefaultCap = 1024

	// AllTopics selects every topic when subscribing.
	AllTopics = ""
)

var (
	ErrConflict = errors.New("eventbus: topic advanced")
	ErrNoTopic  = errors.New("eventbus: topic required")
)

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

// yieldID generates a new ID for the next event.
// IDs look sequential for debuggability, but the values themselves are opaque
// and could be replaced by any other unique identifier scheme.
func (b *Bus) yieldID() string {
	if len(b.events) == 0 {
		return "1"
	}

	v, err := strconv.ParseUint(b.events[len(b.events)-1].ID, 10, 64)
	if err != nil {
		panic("eventbus: invalid id")
	}

	return strconv.FormatUint(v+1, 10)
}

func (b *Bus) filter(q Query) []Event {
	var since time.Time
	if q.AfterID != "" {
		if e := b.lookup(q.AfterID); e != nil {
			since = e.Timestamp
		}
	}

	events := make([]Event, 0, len(b.events))
	for _, e := range b.events {
		if q.Topic != "" && e.Topic != q.Topic {
			continue
		}
		if q.Type != "" && e.Type != q.Type {
			continue
		}
		if !q.Since.IsZero() && !e.Timestamp.After(q.Since) {
			continue
		}
		if !q.Until.IsZero() && !e.Timestamp.Before(q.Until) {
			continue
		}
		if !since.IsZero() && !e.Timestamp.After(since) {
			continue
		}
		events = append(events, e)
	}

	return events
}

// lookup searches by ID starting from the end because
// recent events are more likely to be referenced.
func (b *Bus) lookup(id string) *Event {
	if id == "" {
		return nil
	}

	for i := len(b.events) - 1; i >= 0; i-- {
		if b.events[i].ID == id {
			e := b.events[i]
			return &e
		}
	}

	return nil
}

// Query configures how events are selected when calling Events.
type Query struct {
	Topic   string
	Type    string
	Since   time.Time
	Until   time.Time
	AfterID string
}

// ForEachEvent calls fn with each event that matches q.
// Zero values in q disable their corresponding filters.
func (b *Bus) ForEachEvent(q Query, fn func(Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range b.filter(q) {
		fn(e)
	}
}

// Subscribe registers a new subscriber for a topic. fromID controls the replay window:
// events with ID greater than fromID are replayed before live delivery begins.
// bufferSize configures the subscriber's channel capacity.
func (b *Bus) Subscribe(topic string, fromID string, bufferSize int) (*Subscription, error) {
	if bufferSize <= 0 {
		bufferSize = DefaultCap
	}
	sub := &subscriber{
		topic: topic,
		ch:    make(chan Event, bufferSize),
	}

	b.mu.Lock()
	history := b.filter(Query{
		Topic:   topic,
		AfterID: fromID,
	})
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

	if len(history) > 0 {
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

// Publish appends a new event if topic has not advanced beyond afterID.
// topic/eventType/payload describe the event, afterID is the last known ID for that topic.
// If another event with the same topic was added after afterID, ErrConflict is returned.
// Subscribers drop events when their channel buffer is full.
// Returns the ID assigned to the appended event on success.
func (b *Bus) Publish(topic, eventType string, afterID string, payload any) (string, error) {
	if topic == "" {
		return "", ErrNoTopic
	}

	e := Event{
		Topic:     topic,
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.filter(Query{Topic: topic, AfterID: afterID})) > 0 {
		return "", ErrConflict
	}

	e.ID = b.yieldID()
	b.events = append(b.events, e)

	for sub := range b.subscribers {
		if sub.topic != AllTopics && sub.topic != e.Topic {
			continue
		}

		select {
		case sub.ch <- e:
		default:
			// buffer full: drop for this subscriber
		}
	}

	return e.ID, nil
}

func (b *Bus) Start() string {
	return ""
}

func (b *Bus) End() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.events) == 0 {
		return ""
	}

	return b.events[len(b.events)-1].ID
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
