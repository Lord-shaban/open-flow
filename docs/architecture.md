# Architecture

Status: M1 implements the free image studio with Kafka and Temporal required for system-design training. Owner-scoped encrypted credentials/prompts, verified model discovery, durable image jobs, private S3 storage, signed downloads and the Next.js workspace are implemented. See [free providers](providers/free-providers.md), [API](api.md) and [durable pipeline](persistence.md) for behavior and limits. Video, multi-user authorization, usage billing and automatic routing are planned.

## Modules and responsibilities

Next.js supplies the web client and a same-origin proxy with a signed HttpOnly owner session. A Go module supplies independent API, Temporal worker, dispatcher, outbox relay and example consumer processes. PostgreSQL is the application source of truth. Temporal owns durable orchestration. Kafka owns lifecycle event distribution and replay. Private SeaweedFS S3 holds artifacts; no managed cloud account is required.

```mermaid
flowchart LR
  User[Browser / API client] --> API[Go API: authorization]
  API --> PG[(PostgreSQL jobs + dispatch intent)]
  PG --> Dispatch[Workflow dispatcher]
  Dispatch --> Temporal[Temporal durable execution]
  Temporal --> Worker[Go activity workers]
  Worker --> Router[Capability and policy router]
  Router --> Providers[AI Horde / Workers AI / local adapters]
  Worker --> S3[(Private S3 / R2)]
  Worker --> PG
  PG --> Outbox[Transactional outbox relay]
  Outbox --> Kafka[Kafka lifecycle topics]
  Kafka --> Usage[Inbox-backed training projection]
  Kafka -. planned .-> Analytics[Usage / analytics consumers]
  Usage --> PG
```

## Durable generation flow

```mermaid
sequenceDiagram
  participant U as Client
  participant A as API
  participant D as PostgreSQL
  participant T as Temporal
  participant W as Activity worker
  participant P as Provider
  participant K as Kafka
  U->>A: POST /v1/generations + Idempotency-Key
  A->>D: Transaction: owner-scoped job + dispatch intent
  A-->>U: 202 Accepted + generation ID
  A->>T: Dispatcher: stable workflow ID
  T->>W: Submit activity
  W->>P: One permitted submission
  P-->>W: Result or operation ID
  W->>D: Transaction: attempt / operation / outbox
  loop Pending operation
    T->>T: Durable timer
    T->>W: Poll activity using saved operation ID
    W->>P: Poll operation
    W->>D: Update projection and outbox
  end
  W->>D: Save artifact references + completed event
  D->>K: Relay outbox, keyed by generation ID
  K->>D: Usage consumer: deduplicate event and persist effects
  U->>A: GET /v1/generations/{id}
  A-->>U: Projection + authorized download
```

## Correctness boundaries

Database acceptance and Temporal start are separate commits. Persist dispatch intent then retry starting a deterministic workflow ID with explicit conflict/reuse policy. Do not return a new job when an owner-scoped idempotency key already exists; different payload is 409.

Activities are at least once. Submission may produce an external side effect before a timeout; persist intent, use provider idempotency if supported, disable unsafe submission retries and enter reconciliation_required when acceptance is unknown. A crash between provider acceptance and operation persistence is not solved by Temporal. Persist operation identifiers and resume polling, never resubmit to recover a pending video.

Workflow code is deterministic: no I/O, wall-clock, randomness or goroutines outside SDK constructs. Use activities, SDK timers, workflow-safe concurrency and patch/versioning for changes. Use continue-as-new if polling history grows; cancellation distinguishes request, provider support and confirmed completion.

Job states: queued, submitting, running, reconciliation_required, cancel_requested, succeeded, failed, canceled. Terminal transitions must be monotonic and owned. Events and provider metadata never carry plaintext keys. Temporal history carries IDs; fetch prompts and credentials inside activities to avoid persisted sensitive payloads.

## Event delivery

Transactional outbox bridges state changes and Kafka. Publish at least once; consumers persist event IDs with effects before committing offsets. Partition by aggregate ID for per-job ordering; there is no global ordering. Include schema version, event ID, generation ID, event type, aggregate sequence and occurrence timestamp. Handle duplicate/out-of-order events explicitly, and bound poison-message retries with audited dead-letter/redrive behavior. Do not log prompts or signed media links.

## Routing, security and scaling

M1 requires explicit provider/model selection and verifies discovered IDs before acceptance. Default AI Horde anonymous access; Workers AI requires an encrypted, explicitly confirmed Workers Free connection. ComfyUI is optional local inference and Training canvas is procedural, not AI. Gemini generation is blocked. Unknown queue/quota availability is explicit; route reasons are returned with jobs. No automatic failover, paid fallback or credential cycling is implemented. Multi-provider scoring and health routing remain M3.

Credentials require authenticated encryption with owner/provider AAD and versioned wrapping keys, plus authorization before ingestion. Objects are private and downloads short-lived. Restrict media fetch hosts, byte/MIME limits and telemetry cardinality.

API, workers, dispatch/relay and Kafka consumer groups can scale independently. Exercises will measure queue backlog, workflow latency, consumer lag, outbox age and provider concurrency. M0 Compose is a single-node learning environment, not production HA. See the ADRs and `docs/system-design-labs.md`.
