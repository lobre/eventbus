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

// AllTopics selects every topic when subscribing or in queries.
const AllTopics = "*"

var (
	// ErrConflict is returned when you use Publish with a stale lastID for that topic.
	ErrConflict = errors.New("eventbus: topic advanced")

	// ErrNoTopic is returned when you publish or subscribe with an empty topic.
	ErrNoTopic = errors.New("eventbus: topic required")

	// ErrInvalidBuffer is returned when a negative buffer size is provided.
	ErrInvalidBuffer = errors.New("eventbus: invalid buffer size")
)

// Event is the unit that gets stored and published.
// Events are immutable once appended to the bus.
type Event struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Topic identifies the stream this event belongs to. It is typically a
	// stable key such as an aggregate ID or logical stream name.
	Topic string `json:"topic"`

	// Type describes the kind of event within a topic, and is commonly used
	// for routing and deserialization decisions.
	Type string `json:"type"`

	// Payload holds the event data.
	// It is typically a concrete, JSON-serializable struct value that callers
	// type-assert when consuming events.
	Payload any `json:"payload"`
}

// Subscription exposes an events channel plus a Close function to stop delivery.
//
// Delivery is best-effort: if the subscriber cannot keep up and its channel
// buffer fills up, events for that subscriber are silently dropped.
type Subscription struct {
	C     <-chan Event
	Close func()
}

type subscriber struct {
	topic string
	ch    chan Event
}

// Query configures how events are selected when reading from the log.
//
// Zero values disable their corresponding filters: an empty Topic (or
// AllTopics) selects all topics, an empty Type selects all types, and a zero
// time for Since or Until disables that time bound.
type Query struct {
	// Topic restricts the query to events with this topic.
	// An empty value or AllTopics selects all topics.
	Topic string

	// Type restricts the query to events with this type.
	// An empty value selects all types.
	Type string

	// Since selects events whose timestamp is strictly after this time.
	// A zero value disables the lower time bound.
	Since time.Time

	// Until selects events whose timestamp is strictly before this time.
	// A zero value disables the upper time bound.
	Until time.Time

	// AfterID selects events strictly after the event with the given ID.
	// If no event with that ID exists, AfterID is ignored.
	AfterID string

	// PayloadFilter selects events whose payload satisfies the predicate.
	// A nil value disables payload filtering.
	PayloadFilter func(any) bool
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
		} else {
			// unknown AfterID: treat as no results
			return nil
		}
	}

	events := make([]Event, 0, len(b.events))
	for _, e := range b.events {
		if q.Topic != "" && q.Topic != AllTopics && e.Topic != q.Topic {
			continue
		}
		if q.Type != "" && e.Type != q.Type {
			continue
		}
		if q.PayloadFilter != nil && !q.PayloadFilter(e.Payload) {
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

// ForEachEvent calls fn with each event that matches q.
//
// Zero values in q disable their corresponding filters, as described on Query.
// The set of matching events is determined at the time of the call; new events
// appended after ForEachEvent begins are not passed to fn.
func (b *Bus) ForEachEvent(q Query, fn func(Event)) {
	b.mu.Lock()
	events := b.filter(q)
	b.mu.Unlock()

	for _, e := range events {
		fn(e)
	}
}

// Subscribe registers a new subscriber for a topic.
//
// topic must be non-empty. To subscribe to all topics, use AllTopics.
// If topic is empty, Subscribe returns ErrNoTopic.
//
// fromID is an exclusive lower bound: events with ID greater than fromID are
// replayed to the subscriber before live delivery begins. Using Start() as
// fromID replays all existing events; using End() replays no existing events
// and only delivers new ones.
//
// Delivery is best-effort: if the subscriber's channel buffer is full, both
// replayed events and live events for that subscriber are silently dropped.
// Default buffer size is 1024.
//
// The returned Subscription's Close function unregisters the subscriber and
// closes the events channel.
func (b *Bus) Subscribe(topic string, fromID string) (*Subscription, error) {
	return b.SubscribeWithBufferSize(topic, fromID, 1024)
}

// SubscribeWithBufferSize registers a new subscriber and configures its buffer size.
//
// Same as Subscribe but lets you choose the buffer size. Returns ErrInvalidBuffer
// when bufferSize is negative. A bufferSize of 0 creates an unbuffered channel.
func (b *Bus) SubscribeWithBufferSize(topic string, fromID string, bufferSize int) (*Subscription, error) {
	if topic == "" {
		return nil, ErrNoTopic
	}

	if bufferSize < 0 {
		return nil, ErrInvalidBuffer
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

// Publish appends a new event for a topic if the topic has not advanced
// beyond lastID.
//
// topic and eventType describe the event. lastID is an exclusive lower
// bound for this topic: Publish requires that there are no events with the
// same topic and an ID greater than lastID.
//
// Using Start() as lastID publishes only if no events with this topic exist
// in the bus yet. Using End() as lastID appends after the current end of the bus.
//
// If another event with the same topic was added after lastID, Publish
// returns ErrConflict and does not append. If topic is empty, Publish
// returns ErrNoTopic and does not append.
//
// Subscribers receive the new event on a best-effort basis: if a subscriber's
// channel buffer is full, the event is silently dropped for that subscriber.
//
// On success, Publish returns the ID assigned to the new event.
func (b *Bus) Publish(topic, eventType string, payload any, lastID string) (string, error) {
	return b.publish(topic, eventType, payload, lastID, true)
}

// PublishUnstored delivers an event to subscribers without appending it to the log.
func (b *Bus) PublishUnstored(topic, eventType string, payload any) error {
	_, err := b.publish(topic, eventType, payload, "", false)
	return err
}

func (b *Bus) publish(topic, eventType string, payload any, lastID string, store bool) (string, error) {
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

	if store && len(b.filter(Query{Topic: topic, AfterID: lastID})) > 0 {
		return "", ErrConflict
	}

	if store {
		e.ID = b.yieldID()
		b.events = append(b.events, e)
	}

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

// Start returns the logical lower bound ID of the bus.
//
// Start represents a position before the first event. It currently returns
// the empty string, which callers can pass as fromID or lastID when they
// want to treat the beginning of the bus as their lower bound.
func (b *Bus) Start() string {
	return ""
}

// End returns the ID of the last event in the bus.
//
// If the bus is empty, End returns the empty string. Callers can pass the
// value returned by End as fromID when subscribing, or as lastID when
// publishing, to treat the current end of the bus as their lower bound.
func (b *Bus) End() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.events) == 0 {
		return ""
	}

	return b.events[len(b.events)-1].ID
}

// Dump writes a JSON snapshot of all events to w.
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
//
// The new events take effect atomically with respect to subscribers, but no
// notifications are sent: subscribers are not rewound or updated.
//
// Load trusts the IDs in the input. Future calls to Publish rely on those IDs
// being unique and parseable; invalid or non-sequential IDs may cause Publish
// to panic when generating the next ID.
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

// SaveToFile dumps all events to the given path as JSON, overwriting the file
// if it exists. The write is a snapshot and does not affect subscribers.
func (b *Bus) SaveToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return b.Dump(f)
}

// NewFromFile creates a new Bus and loads events from the given JSON file.
//
// If the file does not exist, NewFromFile returns an empty bus and a nil error.
// If the file exists but cannot be decoded, an error is returned.
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
