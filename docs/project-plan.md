# Open Flow project plan

Open Flow is an open-source, free-first image studio and system-design learning platform with bring-your-own credentials. The product name is **Open Flow**; repository and executable names use `open-flow`. The user's no-payment/no-card requirement supersedes the original Gemini-first plan; hosted paid generation stays disabled.

## First delivery

Establish M0 before implementing media generation: governance, documented architecture, GitHub labels/milestones/issues, Go HTTP foundation, Next.js workspace, Kafka/Temporal local infrastructure, and CI. All later milestones remain open. No real provider calls or credential ingestion ship in M0.

## Product outcomes

1. Create through AI Horde without a card/account; connect optional Workers AI Free or local ComfyUI securely.
2. Discover eligible image/video models and their actual capabilities.
3. Submit durable jobs through a unified API or playground.
4. Resume long-running operations, store private results, and explain failures.
5. Route among explicitly authorized providers with observable decisions.
6. Measure usage, latency, reliability, and cost uncertainty.

## Delivery milestones

### M0 · Project foundation

Repository governance, architecture, contributor onboarding, CI, and runnable Go/Next.js foundation. No paid provider calls.

- **OF-001** — Establish repository governance and contribution standards
- **OF-002** — Document system architecture and initial ADRs
- **OF-003** — Bootstrap Go API and provider contract
- **OF-004** — Bootstrap Next.js workspace and automated quality gates

### M1 · Free image studio

Secure single-owner credentials, verified free-provider discovery, durable image jobs, private storage, and a Flow-inspired playground. AI Horde and Workers AI ship in M1; Gemini discovery remains available but generation is blocked. This is the accepted scope change on 2026-09-29.

- **OF-005** — Add PostgreSQL persistence, transactional outbox and workflow dispatch
- **OF-006** — Implement single-owner access and encrypted BYOK credentials
- **OF-007** — Implement free-provider adapters and verified model discovery; retain Gemini discovery only
- **OF-008** — Implement asynchronous free image generation API
- **OF-009** — Persist generated media in private S3-compatible storage
- **OF-010** — Build provider onboarding and image playground

### M2 · Video and resilience

Restart-safe video operations, retries, cancellation and usage accounting, conditional on a verified free/no-card provider. Basic image history and details are already implemented in M1. Paid Veo is not activated by this plan.

- **OF-011** — Implement restart-safe Veo video operations
- **OF-012** — Add failure classification, retry budgets, and cancellation
- **OF-013** — Build generation history, job details, and usage ledger

### M3 · Multi-provider routing

Vertex AI and one community adapter, observable routing policies, health and quotas.

- **OF-014** — Add Vertex AI adapter with ADC and workload identity
- **OF-015** — Implement explainable routing policies and health signals
- **OF-016** — Add a community provider adapter and conformance harness

### M4 · Production hardening

Multi-user authorization, observability, deployment, load testing, release and recovery procedures.

- **OF-017** — Instrument metrics, traces, and operational dashboards
- **OF-018** — Harden multi-user authorization and security boundaries
- **OF-019** — Add production deployment, recovery, and load verification
- **OF-020** — Publish versioned API reference and first release process

## Definition of done

An issue closes only when its acceptance criteria are met, meaningful automated checks pass, documentation reflects implemented behavior, and the linked change is reviewable. M0 is a foundation, not a production-ready media service. Paid provider integration tests are opt-in; CI uses deterministic fixtures.

## GitHub workflow

GitHub Issues and milestones are the authoritative execution record. `docs/backlog.json` is the initial seed, not a competing live tracker. Each issue has context, dependencies, acceptance criteria, and labels. Use `codex/<short-topic>` or `feat/<short-topic>` branches, focused conventional commits, and PRs referencing issues. No arbitrary deadlines before measuring delivery.

## Risks and decisions

Provider capabilities, billing and regional access change: verify current official documentation at implementation time. Submission timeouts can mean a paid request was accepted: record uncertainty and reconcile rather than resubmit. Project-scoped quotas cannot be solved by rotating keys. Security/ownership must ship before credentials or generation endpoints. Kafka and Temporal are required system-design learning infrastructure; review ADRs for their correctness boundaries.
