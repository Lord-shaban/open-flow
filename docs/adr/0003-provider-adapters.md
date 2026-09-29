# ADR 0003: Provider-neutral media operation contract

- Status: accepted
- Date: 2026-09-29

## Decision

Adapters implement discovery, connection health, submission, operation polling and best-effort cancellation. Models declare image/video generation explicitly. Normalize provider errors with category, Retry-After and whether acceptance is known. Use a registry; routing consumes metadata and interfaces.

Gemini Developer API is primary. Vertex AI is a separate adapter with project/location and ADC/workload identity. Future fal, Replicate and Hugging Face implementations must pass the same contract fixtures.

## Consequences

Common code contains no vendor SDK objects or public credentials. Streaming, edits, seed/duration/aspect-ratio options need explicit capability metadata rather than silent approximation. Dynamic plugins are not loaded from arbitrary binaries in the first release.
