# Unified API

The authoritative contract is `api/openapi.yaml`.

Implemented in M0:

- `GET /healthz`: process liveness, JSON; does not probe future infrastructure.
- `GET /readyz`: HTTP foundation ready; metadata states dependency checks are not configured.
- `GET /v1`: service, foundation version and implemented capabilities (empty).

Responses include a server-generated X-Request-ID. Errors return a structured error code, message and request ID. Unknown routes return 404; unsupported methods return 405.

```sh
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/v1
```

The following are designs, not implemented endpoints: protected credential create/test/revoke, model discovery, POST /v1/generations with Idempotency-Key (202), list/get/cancel generations, usage and settings. They will be added to OpenAPI when implemented.

Generation idempotency is owner-scoped and compares canonical request hashes; same key/different request is 409. Owners can only access their own jobs and artifacts. An SDK can later be generated from the versioned contract.
