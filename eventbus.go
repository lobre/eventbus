package eventbus

import (
	"encoding/json"
	"errors"
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

// Subscription exposes an events channel plus a Close function to stop delivery.
type Subscription struct {
	C     <-chan Event
	Close func()
}

type subscriber struct {
	topic string
	ch    chan Event
}

const BufferDefault = 1024

var ErrConflict = errors.New("eventbus: topic advanced")

// Bus is an in-memory pub/sub bus with an append-only event log.
type Bus struct {
	mu          sync.Mutex
	events      []Event
	subscribers map[*subscriber]struct{}
	lastID      uint64
}

// New creates a Bus with an empty event log.
func New() *Bus {
	return &Bus{
		events:      make([]Event, 0),
		subscribers: make(map[*subscriber]struct{}),
		lastID:      0,
	}
}

// ForEachEvent calls fn for every event in the log and returns the last ID visited.
// If topic is non-empty, only events with that topic are processed.
func (b *Bus) ForEachEvent(topic string, fn func(Event)) uint64 {
	b.mu.Lock()
	events := append([]Event(nil), b.events...)
	b.mu.Unlock()

	var last uint64

	for _, e := range events {
		if topic == "" || e.Topic == topic {
			fn(e)
			last = e.ID
		}
	}

	return last
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
		b.lastID = 0
	} else {
		b.lastID = b.events[len(b.events)-1].ID
	}
	b.mu.Unlock()

	return nil
}

// Subscribe registers a new subscriber for a topic. fromID controls the replay window:
// events with ID greater than fromID are replayed before live delivery begins.
// bufferSize configures the subscriber's channel capacity.
func (b *Bus) Subscribe(topic string, fromID uint64, bufferSize int) (*Subscription, error) {
	if bufferSize <= 0 {
		bufferSize = BufferDefault
	}
	sub := &subscriber{
		topic: topic,
		ch:    make(chan Event, bufferSize),
	}

	var history []Event

	b.mu.Lock()
	history = make([]Event, 0, len(b.events))
	for _, e := range b.events {
		if (topic == "" || e.Topic == topic) && e.ID > fromID {
			history = append(history, e)
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
func (b *Bus) Publish(topic, eventType string, afterID uint64, payload any) (uint64, error) {
	e := Event{
		Topic:     topic,
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for i := len(b.events) - 1; i >= 0; i-- {
		if b.events[i].Topic == topic {
			if b.events[i].ID > afterID {
				return 0, ErrConflict
			}
			break
		}
	}

	b.lastID++
	e.ID = b.lastID
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

	return e.ID, nil
}

func (b *Bus) LastID() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastID
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
