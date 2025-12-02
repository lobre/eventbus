# eventbus

> /!\ I realized I was using the wrong patterns here, not especially adapted for backpressure with this push-model on the read side, and optimitic concurrency on the write side. I am stopping the experimentation and will probably start over with better ideas.

eventbus is a single Go file that keeps an in-memory event log and lets you experiment with CQRS and event sourcing without setting up infrastructure. It has just enough features to publish events, subscribe to them, iterate the log, and dump/load snapshots. Everything else lives in the examples.

This is a demo library. It is not designed for production: events are only stored in memory unless you call `Dump`, and there is no clustering or durability story.

## Basic publish/subscribe (`examples/basic`)

`Publish(topic, eventType, payload, lastID)` appends an event if no newer event exists for that topic. Pass the last event ID you observed for that topic (e.g., from `ForEachEvent` or a previous publish), or use `lastID = bus.End()` when you simply want to append at the current end. `PublishUnstored` delivers without writing to the log. `Subscribe(topic, fromID)` replays events whose IDs sort after `fromID` before streaming live ones, so both aggregates and read models always know where they stand. Use `eventbus.AllTopics` to subscribe to every topic.

## Buffer tuning (`examples/buffer`)

Each subscriber owns its buffer. A size of `1` keeps only the newest value (“state changed”), while larger buffers collect bursts. Publishers never block: when a subscriber’s buffer fills, new events for that subscriber are dropped. Use `SubscribeWithBufferSize` to choose the buffer size (default is 1024 via `Subscribe`).

## Live projections (`examples/projection`)

Subscribe early and keep derived state (e.g., orders per user) in memory. Pass `fromID = bus.Start()` at startup to replay everything, or `fromID = bus.End()` if you only want live updates. The projection example demonstrates a long-lived read model fed by the subscription channel.

## Aggregates (`examples/aggregate`)

`ForEachEvent(query, fn)` walks the current log and calls `fn` for each matching event. Aggregates use `eventbus.Query{Topic: ...}` to rebuild state, capture the last ID they saw, and then call `Publish` with that ID to ensure no newer event slipped in. The aggregate example shows a basic command-side check before emitting a new event.

## Persistence helpers (`examples/persist`)

`Dump`/`Load` work with `io.Writer` and `io.Reader` so you can snapshot or restore wherever you like. `SaveToFile` and `NewFromFile` are thin wrappers that target plain files.

## Sink (`examples/sink`)

Subscribing to `eventbus.AllTopics` receives every event. You can forward that stream anywhere; the sink example appends each event to a file, acting as a simple audit log.

## Server-sent events (`examples/sse`)

`Subscribe(topic, fromID)` aligns with SSE’s `Last-Event-ID`: read the header, pass it as `fromID`, and write each event with its ID so clients can reconnect without missing anything.

## CQRS loop (`examples/cqrs`)

Commands append events (after rebuilding aggregates via `ForEachEvent`), projections rebuild state via subscriptions, and queries read from the projection. The example stitches those parts together into a small CQRS pipeline.

## Running the examples

Each directory under `examples/` is a standalone `go run` program. For example:

```bash
go run ./examples/basic
```

Pick the example that matches what you want to explore.
