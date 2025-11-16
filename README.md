# eventbus

eventbus is a single Go file that keeps an in-memory event log and lets you experiment with CQRS and event sourcing without setting up infrastructure. It has just enough features to publish events, subscribe to them, iterate the log, and dump/load snapshots. Everything else lives in the examples.

This is a demo library. It is not designed for production: events are only stored in memory unless you call `Dump`, and there is no clustering or durability story.

## Basic publish/subscribe (`examples/basic`)

`Publish` appends a Go value to the log and delivers it over channels. `Subscribe(topic, bufferSize)` hands back a receive-only channel plus a `Close` function. `NewEvent` stamps timestamps and numeric IDs so you don’t repeat boilerplate. This is the core API.

## Buffer tuning (`examples/buffer`)

Each subscriber owns its buffer. A size of `1` keeps only the newest value (“state changed”), while larger buffers collect bursts. Publishers never block: when a subscriber’s buffer fills, new events for that subscriber are dropped. The example shows how different sizes behave.

## Live projections (`examples/projection`)

You can hold derived state in memory by subscribing to the topic you care about and updating a struct whenever events arrive. Starting the subscription early ensures you see every event; if you restart later, you can use `SubscribeFromID` to catch up. The example counts orders per user.

## Offline log scans (`examples/report`)

`ForEachEvent(topic, fn)` copies the log and calls `fn` for each event. This fits ad-hoc queries where you just want to walk the log once and exit. The example totals expenses tagged as “food”.

## Persistence helpers (`examples/persist`)

`Dump`/`Load` work with `io.Writer` and `io.Reader` so you can snapshot or restore wherever you like. `SaveToFile` and `NewFromFile` are thin wrappers that target plain files.

## Audit log sink (`examples/auditlog`)

Subscribing to the empty topic (`""`) receives every event. You can forward that stream anywhere; the example appends each event to a file, acting as a simple audit log.

## Server-sent events (`examples/sse`)

`SubscribeFromID(topic, buffer, lastID)` streams events newer than `lastID` and then keeps going. This lines up with SSE’s `Last-Event-ID` header: read the header, pass it to the bus, and write the unified stream back to the client.

## CQRS loop (`examples/cqrs`)

Commands append events, projections rebuild state, and queries read from the projection. The example stitches those parts together into a small CQRS pipeline.

## Running the examples

Each directory under `examples/` is a standalone `go run` program. For example:

```bash
go run ./examples/basic
```

Pick the example that matches what you want to explore.
