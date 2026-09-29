# Deployment

Status: M1 containers implement the local free image studio. Public production deployment remains gated by M4.

The API Dockerfile produces a non-root minimal image with API, worker, migration, pipeline and storage cleanup binaries. The web Dockerfile builds Next.js standalone output. `infra/compose.yaml` offers loopback-bound PostgreSQL, Kafka, Temporal and authenticated private SeaweedFS S3. The app profile runs real image workflows, dispatch, outbox relay and projection consumer. Generate local secrets with `pnpm setup:local`; see [development](development.md) and [free-provider requirements](providers/free-providers.md).

M1 includes single-owner authorization, encrypted credentials/prompts, private object storage and checksummed migrations. Before public hosting, complete multi-user authorization, TLS/reverse proxy with secure cookies, least-privilege database/storage accounts, egress limits, metrics and recovery drills. Back up versioned encryption keys alongside the database and objects. Production secrets must come from a secret store, not baked image layers or committed env files. Pin image digests during release and scan dependencies. No hosted paid deployment has been provisioned.

API and workers can run independently around the persistent dispatch queue. Kafka and Temporal are required learning infrastructure; the supplied single-node stack has no HA guarantee. Add Redis caching or Kubernetes only with a clear exercise and reviewed ADR. OF-019 will deliver verified production procedures.
