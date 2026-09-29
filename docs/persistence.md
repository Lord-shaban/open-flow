# Durable job pipeline (OF-005)

Implemented: PostgreSQL migrations, internal persisted learning jobs, Temporal dispatch, transactional outbox, Kafka relay and an inbox-backed job-event projection. No provider calls, credential ingestion, public generation endpoints or billing effects are enabled by this slice. Those follow OF-006/007/008/013.

```mermaid
sequenceDiagram
  participant C as Learning command
  participant D as PostgreSQL
  participant T as Temporal
  participant K as Kafka
  participant P as Projection consumer
  C->>D: Transaction: job + dispatch intent + queued event
  C->>T: Dispatcher: start open-flow.job.UUID
  T-->>C: Run ID, or existing run
  C->>D: Acknowledge dispatch
  T->>D: Activity transaction: succeeded + outbox event
  D->>K: Relay oldest pending event per job
  K-->>D: Publish acknowledgement
  D->>D: Mark outbox published
  K->>P: Fetch message (offset still uncommitted)
  P->>D: Transaction: inbox + projection effect
  P->>K: Commit offset
```

## Schema and ownership

Migration `001_pipeline` creates owners, encrypted credential records, jobs, attempts, artifacts, usage, outbox, consumer inbox and job-event projections. Credential encryption is a schema contract only; the cryptography and authorized credential API are OF-006. No plaintext credential or prompt column is introduced.

Jobs are owned, and composite foreign keys prevent attaching another owner's credential, attempt, artifact or usage record. Owner reads require both owner and job IDs. Application tables use UUIDs, constrained states, UTC-capable timestamps, indexed foreign keys and partial queue indexes. Usage stores decimal amounts with explicit currency and estimated/reported distinction; nothing writes billable usage yet.

The migrations are embedded in the Go binary and run explicitly, never on API startup. A transaction-scoped advisory lock serializes migration processes; the recorded SHA-256 checksum rejects changed applied SQL. Up and down are both transactional. Down drops these tables and their data; the CLI requires `down --allow-data-loss`. Preserve an applied migration and add a new migration for future schema changes.

## Run locally

Start the infrastructure without the app profile, then initialize the three-partition job topic:

```sh
docker compose -f infra/compose.yaml up -d --wait postgres temporal-postgres temporal temporal-ui kafka
docker compose -f infra/compose.yaml run --rm kafka-jobs-init
```

Set `OPEN_FLOW_DATABASE_URL` in each Go terminal. The development value from `.env.example` is:

```sh
export OPEN_FLOW_DATABASE_URL='postgres://open_flow:local-development-only@127.0.0.1:5432/open_flow?sslmode=disable'
```

PowerShell equivalent:

```powershell
$env:OPEN_FLOW_DATABASE_URL = 'postgres://open_flow:local-development-only@127.0.0.1:5432/open_flow?sslmode=disable'
```

These credentials are local placeholders. Use a properly encoded PostgreSQL connection string for overrides. Go reads process environment, not the root `.env` file.

From `services/api`, run `go run ./cmd/migrate up`, then `go run ./cmd/worker` in another terminal with the same environment. Run `go run ./cmd/pipeline-smoke` to exercise the full pipeline and failure windows. Keep regular dispatch/relay processes stopped during fault injection so they cannot acknowledge the probe's rows first. The probe writes a new owner/job and projection for inspection; it makes no paid calls. Inspect `open-flow.job.<job UUID>` in Temporal UI.

For continuous processing, the separate commands are:

```sh
go run ./cmd/pipeline dispatch
go run ./cmd/pipeline relay
go run ./cmd/pipeline consume
```

Each runs in its own terminal. The app Compose profile also starts the migration, worker, dispatcher, relay and consumer with ordered dependencies. The API still exposes only its foundation routes; ordinary image/video jobs cannot be dispatched by the probe dispatcher.

## Recovery and ordering

Dispatch and relay each hold one row lock during a network operation, with an eight-second operation deadline. `FOR UPDATE SKIP LOCKED` lets other instances process unrelated rows. This deliberately simple design bounds the lock duration and makes a crashed process release its claim automatically; it consumes a database connection while awaiting the dependency. Larger deployments should measure this cost before moving to leased claims with fencing. There is no long transaction around a provider call or a batch.

Temporal uses explicit `FAIL` conflict and `REJECT_DUPLICATE` reuse policies, with already-started errors surfaced to the dispatcher. A restart reuses the existing run rather than terminating it or starting another run, including after completion. Temporal retains workflow identities only within its history-retention window. A dispatch acknowledgement lost beyond that window requires reconciliation; this slice does not promise permanent exactly-once workflow starts. Record this boundary before adapting the probe to paid generation.

Outbox selection excludes an event while any lower sequence remains unpublished, including a locked or backoff-delayed predecessor. Events use the job ID as Kafka key. Keep the topic partition count fixed while relying on that ordering; a repartitioning migration needs a separate plan. Successful acknowledgement records `published_at`; failures record attempts and exponential backoff capped at 60 seconds. Relay retries continue while the service runs. An uncertain Kafka acknowledgement can produce duplicates, and consumers must handle them.

Inbox identity is scoped by consumer name and event ID. Hash comparison rejects the same event ID with altered content. Inserting the inbox record and updating the projection share a transaction. Duplicate and stale sequences never regress state or repeat effects. Consumer-group offsets are committed synchronously after that transaction. A crash between the DB commit and offset commit replays the inbox safely. The example projection counts accepted transitions; it is not the usage ledger.

Malformed messages, invalid keys, unknown jobs and event collisions stop the consumer without committing the offset. Correct the cause before restarting. Automated dead-letter/redrive behavior and billing consumption remain OF-013; simply skipping a poison message would lose auditability. A gap in sequence is observable through `last_sequence` but this state projection accepts a newer snapshot; a financial ledger must use a stricter completeness policy.

## Verification

`OPEN_FLOW_TEST_DATABASE_URL` enables real PostgreSQL tests under isolated, randomly named schemas. The tests remove only their own schemas. Linux CI runs them with race detection and covers migration up/down/checksum, ownership, concurrent completion, dispatcher acknowledgement loss, persisted backoff, relay ordering under concurrency and cancellation, duplicate inbox writes, stale events and independent consumers.

The infrastructure smoke test connects PostgreSQL, Temporal and Kafka. It cancels the dispatcher after Temporal acknowledges, retries after workflow completion, cancels the relay after Kafka acknowledges and reads the replayed messages through an actual consumer group. Expected output: `workflow_run_reused: true`, `event_sequences: [1,1,2]`, `duplicates_ignored: 1`, `projection_effects: 2`, `status: ok`.

References: [PostgreSQL locking](https://www.postgresql.org/docs/current/explicit-locking.html), [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool), [Temporal start options](https://pkg.go.dev/go.temporal.io/sdk/client#StartWorkflowOptions), [Kafka FetchMessage](https://pkg.go.dev/github.com/segmentio/kafka-go#Reader.FetchMessage).
