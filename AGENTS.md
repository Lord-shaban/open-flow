# Open Flow contributor automation

- Product name: Open Flow. Repository/module prefix: open-flow.
- The project is a system-design learning platform. Kafka and Temporal are explicit requirements from the foundation.
- Temporal orchestrates durable generation workflows; Kafka distributes versioned lifecycle events. PostgreSQL owns application data and transactional outbox/inbox.
- Follow GitHub issues/milestones, docs/project-plan.md and accepted ADRs. M0 is a foundation; do not represent planned provider integrations as working.
- Keep provider SDK types out of core contracts. Never expose provider credentials to frontend code, logs, events, or workflow history.
- Do not blindly retry ambiguous paid submissions or rotate keys to evade provider limits.
- Run pnpm check/build and Go formatting/vet/tests. Linux CI includes race and real infrastructure probes without paid calls.
- Update documentation and contracts with behavior changes; keep commits focused.
