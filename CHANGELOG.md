# Changelog

## Unreleased — M1 free image studio

- Add single-owner API access, HttpOnly browser sessions and versioned AES-GCM encryption for credentials and prompts.
- Add anonymous AI Horde and optional Workers AI Free adapters, verified model discovery, optional local ComfyUI and a clearly labeled non-AI training renderer. Block paid Gemini generation while retaining discovery/testing.
- Add idempotent image acceptance, once-only submission intent, persisted async polling and conservative ambiguous-result handling through Temporal.
- Store bounded validated images in private SeaweedFS S3 with owned signed downloads, 30-day retention, tombstones and fenced garbage collection.
- Add a responsive Google Flow-inspired studio, connection onboarding, model/aspect selection, durable library and private download/details.
- Extend real PostgreSQL/Kafka/Temporal/S3 CI probes and desktop/mobile browser verification without paid provider calls.

## Unreleased — durable pipeline

- Add PostgreSQL schema with transactional migrations, owned jobs and outbox/inbox records.
- Add stable-ID Temporal dispatch, retry-safe persisted probe activities and a Kafka relay/projection consumer.
- Add real database integration tests and a Kafka/Temporal crash-window smoke lab without paid calls.

Changes follow semantic versioning once public releases begin.

## Unreleased

- Establish Open Flow project vision, milestone backlog, governance and ADRs.
- Add a runnable Go HTTP foundation and provider-neutral media contracts.
- Add a Next.js workspace, pinned dependencies, Docker development configuration and CI.
- At foundation delivery, provider connections and media generation were planned; image support now ships in M1 above. Video remains planned.
