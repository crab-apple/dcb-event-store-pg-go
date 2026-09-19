# dcb-event-store-pg-go

**Work in progress.** A Go port of parts of [PaulGrimshaw/dcb-event-store](https://github.com/PaulGrimshaw/dcb-event-store) (a JS/TS implementation of the Dynamic Consistency Boundary event-store pattern). See that repo for background and design rationale.

## Scope

This port includes the event-store layer only. That is:

- The `EventStore` interface
- In-memory implementation
- Postgres implementation
