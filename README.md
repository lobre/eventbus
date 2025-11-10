# eventbus

Minimal in-memory event bus for Go with pub/sub, an append-only event log, and optional JSON file persistence.

## Goals

- Provide a tiny, self-contained event bus for learning CQRS/event-sourcing concepts.
- Keep everything in memory, single process, and goroutine-safe.
- Make it easy to:
  - append events to an in-memory log,
  - publish events to subscribers,
  - subscribe via channels,
  - save/load the event log as JSON.

## Non-goals

- No distribution or clustering.
- No durable, append-only storage beyond a simple JSON snapshot.
- No advanced querying or filtering layer over the event store.
- No consumer groups, load balancing, or delivery guarantees beyond best effort.

## Design choices

- **Event type**: `Event` with `ID`, `Timestamp`, `Topic`, `Type`, and `Payload`.
- **Event store**: `InMemoryEventStore` implementing `EventStore` (`Append`, `All`).
- **Bus**: `InMemoryBus` implementing `Bus` (`Publish`, `Subscribe`, `Events`).
- **Subscriptions**:
  - Each subscriber gets its own buffered channel.
  - If the channel is full, events are dropped for that subscriber (no backpressure).
  - Subscriptions are topic-filtered; empty topic means “receive all events”.
- **Persistence**:
  - `SaveToFile` writes all events as a JSON array.
  - `LoadFromFile` reads events back and can be used to seed a new `InMemoryBus`.

## Structure

- `eventbus.go` — the entire event bus library (single file).
- `cmd/ping/main.go` — minimal example HTTP service using the event bus.

## Usage

1. Initialize the module (from the project root):

```
go mod init github.com/lobre/eventbus
```

2. Build the example command:

```
go build -o ping ./cmd/ping
```

3. Run it:

```
./ping
```

4. In another terminal, publish an event:

```
curl "http://localhost:8080/ping"
```

5. Watch events logged in the server output.

On shutdown (`Ctrl+C`), events are saved to `events.json` and reloaded on the next start.

See `cmd/ping/main.go` for the full usage example.

