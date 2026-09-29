# ADR 0001: Start with a modular monolith

- Status: accepted
- Date: 2026-09-29

## Context

Open Flow needs clear provider boundaries and durable jobs, but has no measured traffic or independent teams yet.

## Decision

Use one Go module, an HTTP API and a future worker sharing domain contracts, with a separate Next.js frontend. PostgreSQL is the application source of truth. Temporal orchestrates generation workflows, Kafka distributes lifecycle events, and S3-compatible storage owns artifacts. Kafka/Temporal are explicit system-design training requirements. Keep modules independent of transport and concrete providers.

## Consequences

This keeps domain code cohesive and makes transactional behavior testable. Processes can scale independently without premature services. Modules may become services only when ownership, load or fault isolation justifies the cost.
