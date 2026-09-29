# ADR 0002: Temporal workflows and Kafka events from the foundation

- Status: accepted
- Date: 2026-09-29

## Context

Open Flow is explicitly a system-design training project. Kafka and Temporal are required to practice durable orchestration and event-driven systems from the beginning.

## Decision

Temporal owns generation workflow execution, durable timers, activity retry policy and cancellation. PostgreSQL owns credentials, API read models, idempotency, usage and transactional outbox. Kafka distributes versioned lifecycle events to independent usage/analytics/audit consumers; it does not serve as a second workflow scheduler.

Persist the accepted job and dispatch intent in one database transaction. A dispatcher starts a Temporal workflow with a stable generation-derived ID and explicit ID conflict/reuse policy. It can retry after a crash without creating a second workflow. Activities update job state and insert events into an outbox atomically. A relay publishes outbox records to Kafka with generation ID as the partition key. Consumers deduplicate by event ID and commit offsets only after durable effects.

Submission activities must not inherit automatic retries blindly: providers may accept a paid request before timing out. Without provider idempotency/reconciliation, disable submission retries and surface an ambiguous state. Temporal does not provide exactly-once external billing. Workflow history contains IDs and minimal metadata; credentials and prompts remain outside history unless a reviewed payload encryption codec is configured.

## Consequences

The local stack is intentionally larger for learning. M0 ships a safe Temporal probe workflow, Kafka envelope/publisher and smoke commands. Generation dispatch, outbox/inbox persistence and consumers ship incrementally in M1/M2. Study replay, rebalances, duplicate events, dead letters and worker crashes through documented labs. Redis remains optional caching; Kubernetes remains a later deployment exercise.
