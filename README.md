# eventbus

A single-file in-memory event bus for Go, intended as a playground for CQRS and event-sourcing ideas.

The implementation favors a tiny API surface — append, iterate, subscribe, dump, and load — while being thread-safe. Everything else (replay helpers, sinks, projections) is shown in `examples/`.

This library is **not** production-ready. It has no distribution or durability story beyond JSON snapshots, but it’s great for experiments and demos.

## Features

- **Publish + subscribe (examples/basic)**  
  `Publish(event)` appends to the log and fans the event out to every `Subscribe(topic, bufferSize)` consumer using Go channels. `Event` instances are created with `NewEvent(topic, type, payload)` and contain `Timestamp`, `Topic`, `Type`, and `Payload`.

- **Buffer tuning (examples/buffer)**  
  Each subscription owns its buffered channel. Small buffers coalesce bursts (“state changed” signals) while larger buffers tolerate spiky traffic. When the buffer is full the publisher drops the event for that subscriber instead of blocking.

- **Checkpoint-friendly projections (examples/projection)**  
  `ForEachEvent(topic, fn)` iterates a snapshot of the log. Keep a counter of how many events you’ve already applied per topic, skip that many on the next replay, and resume without revisiting old events.

- **Replay then stay live (examples/replay)**  
  Combine `ForEachEvent` (for history) with a fresh `Subscribe` (for new events) to rebuild projections and keep them hot without needing extra APIs.

- **JSON persistence (examples/persist)**  
  `Dump(io.Writer)` / `Load(io.Reader)` and the convenience helpers `SaveToFile` / `NewFromFile` make it easy to snapshot or restore the log for demos, backups, or hand-edited fixtures.

- **Global sinks (examples/sink)**  
  Subscribing to the empty topic (`""`) receives every event. You can forward that stream anywhere — for instance append them to a log file for durability — while keeping the bus core unchanged.

- **CQRS loop (examples/cqrs)**  
  Commands mutate aggregates by publishing events, while projections rebuild state using `ForEachEvent` at startup and a live subscription afterward. The example wires an account command handler and balance projection without extra APIs.

## Running the examples

Each directory under `examples/` is a standalone `go run` program. For instance:

```bash
go run ./examples/basic
```

Pick the example that matches the pattern you want to explore.
