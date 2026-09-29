# ADR 0004: Server-side owned credentials and private artifacts

- Status: accepted
- Date: 2026-09-29

## Decision

Ship owner authorization before accepting provider credentials. Encrypt keys with authenticated encryption, per-record nonces, owner/provider AAD and versioned wrapping keys; prefer KMS in production. Never expose plaintext on reads, browser storage, telemetry or error responses. Permit revocation and rotation. Vertex deployments use workload identity rather than uploaded long-lived service-account files.

Objects are private. Validate ownership before issuing short-lived downloads. Retention, key backup/recovery and deletion must be documented.

## Consequences

Local examples contain placeholders only. Loss of encryption keys makes records unrecoverable. M0 contains no credential routes and performs no paid generation.
