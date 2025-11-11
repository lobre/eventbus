# eventbus

A minimal in-memory event bus for Go, intended as a playground for CQRS and event-sourcing patterns.

This library is **not** production-ready: it has no distribution, no durability guarantees beyond simple JSON dumps, and no scalability story. It is, however, very handy for small apps, demos, and experiments.

## Features

- **In-memory event log**  
  The bus keeps an append-only slice of `Event` values in memory. This is your “event store” for replaying history or building projections.

- **Simple event type**  
  `Event` has `Timestamp`, `Topic`, `Type`, and `Payload`.

  Use `NewEvent(topic, type, payload)` to create events with an automatic UTC timestamp, keeping event creation boilerplate low.

- **Pub/sub via Go channels**  
  `Subscribe(topic, bufferSize)` returns a `Subscription` with a `chan Event`.

  Because you get a real channel, you can use `select` to combine events with context cancellation, timers, other channels, etc. This makes it easy to plug the bus into HTTP handlers, background workers, SSE streams, and so on.

- **Configurable buffer size with drop-if-full semantics**  
  Each subscription has its own buffered channel. When the buffer is full, new events for that subscriber are dropped (publishers never block).

  The buffer size acts as a behavior knob:
  - `bufferSize = 1` works well for “refresh” or “state changed” signals where you only care that *something* changed; bursts of such events effectively coalesce into at most one pending event.
  - Larger buffers (e.g. 256 or 1024) are better for logging, debugging, or projections where you want to tolerate bursts and see more of the event stream.

- **Full-log iteration for projections**  
  `ForEachEvent(topic, fn)` iterates over the entire event log (optionally filtered by topic) and calls `fn` for each event.

  This is enough to implement simple projection systems, such as:
  - building in-memory read models, or
  - populating SQL tables optimized for queries, using events as the source of truth and projections as derived state.

- **JSON dump/load via io.Writer/io.Reader**  
  - `Dump(w io.Writer)` writes all events as pretty-printed JSON.
  - `Load(r io.Reader)` replaces the entire log from JSON.

  Because these use `io.Writer` / `io.Reader`, you can:
  - save regular snapshots to files,
  - push backups to a GitHub gist or object storage,
  - or stream logs over HTTP.

  You can trigger dumps on a timer or after every N events from within a subscriber to get simple, ongoing backup behavior.

- **Small, single-file implementation**  
  The entire library lives in `eventbus.go` and is easy to copy into other projects.

## Example

There is a minimal example command under `cmd/ping/main.go` that:

- creates a bus (optionally loading from `events.json`),
- starts a subscriber that logs all events,
- exposes an HTTP endpoint that publishes events.

To build and run it from the project root:

```bash
go build -o ping ./cmd/ping

./ping

curl "http://localhost:8080/ping"
```

You’ll see the events logged in your terminal.
