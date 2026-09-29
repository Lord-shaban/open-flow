# System-design labs

Kafka and Temporal are part of the foundation because the project is designed for engineering practice.

## Foundation probes (M0)

Start Compose infrastructure, then run the worker in one terminal and `go run ./cmd/temporal-smoke` in another. Inspect the workflow in Temporal UI at http://localhost:8088. Run `go run ./cmd/kafka-smoke` to publish and read a uniquely identified foundation event. Neither probe calls a provider, stores credentials or generates media.

## Exercises as implementation ships

OF-005 now includes a runnable persisted-pipeline lab: [setup and recovery](persistence.md). Run `go run ./cmd/pipeline-smoke` with the database-enabled worker. It exercises lost DB acknowledgement after a Temporal start and lost outbox acknowledgement after a Kafka publish, then verifies stable workflow identity, duplicate delivery and one durable consumer effect per event. PostgreSQL tests also prove relay concurrency, delayed predecessor ordering and stale event handling. Provider-specific exercises below remain planned.

| Exercise                                                       | Expected invariant                                         | Milestone |
| -------------------------------------------------------------- | ---------------------------------------------------------- | --------- |
| Kill/restart a Temporal worker during a poll timer             | Resume saved operation; no second submission               | M1/M2     |
| Kill dispatcher after workflow start before DB acknowledgement | Stable workflow ID prevents a second execution             | M1        |
| Crash outbox relay after publish before acknowledgement        | Duplicate event is deduplicated by consumer inbox          | M1/M2     |
| Add a second Kafka usage consumer                              | Rebalance preserves durable usage effects                  | M2        |
| Inject 429 with Retry-After                                    | Bounded backoff honors limits; no key cycling              | M2        |
| Time out provider submission after acceptance                  | Reconciliation state; no blind retry/failover              | M2        |
| Deliver old aggregate sequence after newer event               | Projection ignores stale transition                        | M2        |
| Break an activity repeatedly                                   | Retry budget ends; failure and dead letter observable      | M2        |
| Deploy changed workflow code against saved history             | Replay test passes and version/patch handles compatibility | M2/M4     |
| Stop Kafka while provider completes                            | DB/outbox commits; event is published after recovery       | M1/M2     |

Record hypothesis, fault injection, logs/metrics, invariant and recovery result in the relevant GitHub issue. A single-node dev broker demonstrates semantics, not replication or production availability. Multi-broker, TLS/SASL, backup and disaster recovery belong in M4.
