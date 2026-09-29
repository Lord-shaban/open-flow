# Local development

## Full M1 studio

Requirements: Docker Compose, Node.js 24, pnpm 11.25.0 and Git. Go 1.27 is optional for host-side backend development. Docker Desktop's personal/educational use or an equivalent free Docker Engine installation supplies containers; choose a license appropriate to your environment.

```sh
pnpm install --frozen-lockfile
pnpm setup:local
docker compose --env-file .env -f infra/compose.yaml --profile app up -d --build
```

Setup generates a 32-byte owner token, versioned AES key and random local S3 secret in `.env`. It refuses to overwrite an existing file. Keep it private and retain the encryption keys with database backups. The Go binary reads process environment, not dotenv; Compose injects the relevant variables. The web container receives the owner token server-side only.

Web http://localhost:3000 · API http://127.0.0.1:8080 · Temporal UI http://localhost:8088 · private S3 http://127.0.0.1:8333. All host ports bind to loopback. The app profile starts migrations, API, worker, dispatcher, outbox relay, consumer and web. Kafka and Temporal are required, not optional infrastructure substitutes. Redis is an unused optional cache lab.

Unlock the studio using `OPEN_FLOW_OWNER_TOKEN` from `.env`. AI Horde works anonymously. Anonymous prompts/results may be shared. No provider key is needed for Training canvas, which creates procedural images and is explicitly not AI. For real local inference, install ComfyUI with a compatible checkpoint, set `OPEN_FLOW_COMFYUI_ENDPOINT=http://host.docker.internal:8188`, and recreate the API and worker. Checkpoint licensing and available hardware remain your responsibility.

## Host UI development

Start the Docker infrastructure and backend app services, then run the host web app on an available port. The root `.env` is not loaded by Next automatically:

```sh
node --env-file=.env node_modules/next/dist/bin/next dev apps/web --hostname 127.0.0.1 --port 3001
```

Alternatively, `pnpm dev` previews the interface without a configured owner session. For a production preview after `pnpm build`, use `node --env-file=.env scripts/start-web.mjs` with port 3000 available. Set `OPEN_FLOW_API_URL` to the host Go API URL. Never use `NEXT_PUBLIC_*` for owner or provider secrets.

## Checks

```sh
pnpm check
pnpm build
cd services/api
go fmt ./...
go vet ./...
go test ./...
```

Linux CI runs `go test -race ./...` with a real PostgreSQL service and isolated per-test schemas. `OPEN_FLOW_TEST_DATABASE_URL` enables those tests locally; otherwise they explicitly skip. Windows race detection requires a C compiler. CI also runs real Kafka, Temporal and private SeaweedFS probes, `cmd/image-smoke` for provider failure/idempotency/download/retention boundaries, and Playwright browser tests at desktop and mobile widths against the real backend using Training canvas. Hosted provider HTTP fixtures make CI deterministic and incur no image charges.

After starting the infrastructure, `go run ./cmd/temporal-smoke`, `go run ./cmd/kafka-smoke`, `go run ./cmd/pipeline-smoke` and `go run ./cmd/image-smoke` run from `services/api` with the database/S3 environment configured. The image smoke creates its own private bucket and owner; only its own fixtures are removed. It does not use a real provider key. For local random S3 secrets, export the generated values to the host process.

## Operations

`/healthz` is liveness. Configured `/readyz` checks PostgreSQL and the private S3 bucket; Kafka/Temporal readiness is independently enforced by Compose and probes. Generation acceptance commits a durable dispatch intent even while a worker is unavailable. Stop/start workers to observe recovery; see [system-design labs](system-design-labs.md) and [persistence](persistence.md).

Artifacts expire after 30 days, and removal from the library denies new downloads immediately. To collect expired, deleted and orphan objects, stop API and workers first to fence uploads, then run `storage-gc` (dry run by default). Inspect the count, then run `storage-gc --apply` in a configured one-off container. A 24-hour grace period protects recent objects; only owned-format object keys are considered. Restart API/workers afterwards. Do not run apply concurrently with uploads.

Encryption rotation: add the new 32-byte base64 key to `OPEN_FLOW_ENCRYPTION_KEYS` under a new positive version and set `OPEN_FLOW_ENCRYPTION_VERSION`. Keep old versions so existing credentials and prompts can still decrypt. Restart API/workers, then rotate credentials through Connections. Revoking a key prevents future resolution and queued submission. Changing the owner token invalidates sessions and signed download links. Use HTTPS and `OPEN_FLOW_SECURE_COOKIE=true` when deploying beyond local development; production authentication/HA are M4 work.
