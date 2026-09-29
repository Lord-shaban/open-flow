# Add a provider

1. Create a meaningful issue with official API references, capabilities and billing constraints.
2. Add an adapter under `services/api/internal/provider/<name>` implementing the public internal contracts in `provider.go`.
3. Declare verified output capabilities and normalized model identifiers; model discovery alone is not proof of image/video generation.
4. Resolve credentials only on the server. Set timeouts, response size limits and allowed media download hosts.
5. Normalize authentication, quota, transient, policy and ambiguous acceptance errors. Respect Retry-After; never hide duplicate-billing risk.
6. Support durable operation IDs and poll/resume. Return unsupported cancellation explicitly.
7. Add deterministic contract fixtures and opt-in real provider tests; default CI makes no paid calls.
8. Register the adapter at the composition root. Core routing must work without new vendor-specific branches.
9. Document setup, capabilities, limitations and cost uncertainty, then link the PR to the issue.
