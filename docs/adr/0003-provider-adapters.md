# ADR 0003: Provider-neutral media operation contract

- Status: accepted
- Date: 2026-09-29

## Decision

Adapters implement discovery, connection health, submission, operation polling and best-effort cancellation. Models declare image/video generation explicitly. Normalize provider errors with category, Retry-After and whether acceptance is known. Use a registry; routing consumes metadata and interfaces.

M1 amendment (2026-09-29): the user's free/no-card requirement replaces the original Gemini-first priority. AI Horde is default, Workers AI Free optional, and ComfyUI local inference optional. Training canvas is a procedural test adapter, not AI. Gemini SDK integration remains available for discovery/testing, while generation is blocked. Vertex AI, fal, Replicate and other paid integrations remain inactive and would need an explicitly accepted scope change plus contract fixtures.

## Consequences

Common code contains no vendor SDK objects or public credentials. Streaming, edits, seed/duration/aspect-ratio options need explicit capability metadata rather than silent approximation. Dynamic plugins are not loaded from arbitrary binaries in the first release.
