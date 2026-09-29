# Contributing to Open Flow

Start with an existing GitHub issue or open a proposal with context and acceptance criteria. GitHub is the execution source of truth; the backlog JSON is the initial seed. Good first issues and help wanted labels identify contributor entry points.

1. Read the architecture, relevant ADRs and development guide.
2. Fork/branch from main. Use `feat/<topic>`, `fix/<topic>`, `docs/<topic>` or `codex/<topic>`.
3. Keep commits focused; use Conventional Commits such as `feat(provider): add discovery`.
4. Run `pnpm check`, `pnpm build`, Go formatting, vet and tests. Linux CI runs race detection.
5. Include meaningful tests for failure/durability/security behavior. Use fixtures; paid tests require explicit opt-in.
6. Update documentation and API contracts with behavior changes.
7. Open a PR linking its issue, acceptance evidence, validation and material limitations.

Architecture, provider semantics, credential handling and new infrastructure need reviewed ADRs. Avoid unrelated changes and vendor-specific branching in core modules. Keep dependencies pinned; update lockfiles. Do not commit credentials, generated media, private logs or screenshots containing secrets.

Reviews check behavior, maintainability, accessibility, ownership and recovery. A release requires changelog entries, all checks passing and a documented rollback. Security reports belong in the private disclosure channel.
