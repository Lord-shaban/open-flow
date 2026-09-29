# Local development

## Requirements

Node.js 24 LTS, pnpm 11.25.0, Go 1.27+, Git; Docker Compose is optional for infrastructure. See [Next.js installation](https://nextjs.org/docs/app/getting-started/installation) for framework runtime requirements. Dependency versions and lockfile are pinned in the repository.

```sh
git clone https://github.com/Lord-shaban/open-flow.git
cd open-flow
pnpm install --frozen-lockfile
pnpm dev
```

In a second terminal:

```sh
cd services/api
go run ./cmd/api
```

Web: http://localhost:3000. API: http://127.0.0.1:8080. The web foundation runs independently; it does not imply a connected backend or provider. Health is process readiness only in M0.

## Checks

```sh
pnpm check
pnpm build
cd services/api
go fmt ./...
go vet ./...
go test -race ./...
```

Check Go formatting before commit: `gofmt -l .` should print no filenames. Race testing on Windows may require a C compiler; use ordinary `go test ./...` locally if unavailable and rely on Linux CI for the race gate.

## Environment and infrastructure

Copy `.env.example` to `.env` only if you need overrides. The Go binary reads process environment; it does not automatically load dotenv. Next reads its own `apps/web/.env.local`. Compose consumes the root `.env`.

```sh
docker compose -f infra/compose.yaml up -d
docker compose -f infra/compose.yaml --profile app up --build
```

Compose includes PostgreSQL, a single Kafka KRaft broker, topic initialization, Temporal with its own PostgreSQL database, and Temporal UI. Redis is available through the optional cache profile. Object storage is introduced in OF-009. M0 API readiness checks the HTTP foundation only. Never expose development credentials or host ports to the public network.

Start the learning probes after the infrastructure is healthy:

```sh
cd services/api
go run ./cmd/worker
```

In another terminal:

```sh
cd services/api
go run ./cmd/temporal-smoke
go run ./cmd/kafka-smoke
```

Temporal UI: http://localhost:8088. Kafka's host listener is 127.0.0.1:9092; containers use kafka:29092. The foundation topic has one partition. These probes make no provider calls. See [system-design labs](system-design-labs.md).
