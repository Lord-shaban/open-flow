# Deployment

Status: M0 containers are development foundations; production media deployment is gated by M4.

The API Dockerfile produces a non-root minimal image. The web Dockerfile builds Next.js standalone output. `infra/compose.yaml` offers loopback-bound development services and optional application containers. A Temporal foundation worker exists; real generation workflows and Kafka consumers remain tracked milestones. Kafka/Temporal use development configuration with loopback host ports. Private object storage is introduced in OF-009.

Before public hosting, complete owner/session authorization, encrypted credentials and key backup, private object storage, migration/rollback, TLS/reverse proxy, least-privilege database access, egress limits, metrics and recovery drills. Production secrets must come from the platform secret store, not baked image layers or committed env files. Pin image digests during release and scan dependencies.

Scale API/worker separately after the persistent job queue exists. Kafka and Temporal are required learning infrastructure. Add Redis caching or Kubernetes only with a clear exercise and reviewed ADR. OF-019 will deliver verified production procedures.
