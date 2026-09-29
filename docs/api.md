# M1 API

The [OpenAPI contract](../api/openapi.yaml) describes the Go HTTP service. `/healthz`, `/readyz` and `/v1` are public; `/v1/*` resources require `Authorization: Bearer <owner-token>`. Every resource is scoped to the configured single owner. Errors include a sanitized code/message/request ID; no provider error body, credential, encrypted payload, prompt or object key is returned.

| Method/path                                          | Behavior                                                                                     |
| ---------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| GET `/v1/credentials`                                | Redacted connection metadata                                                                 |
| POST `/v1/credentials`                               | Encrypt BYOK, return 201 metadata                                                            |
| PUT `/v1/credentials/{id}`                           | Rotate an active owned connection, increment revision                                        |
| DELETE `/v1/credentials/{id}`                        | Revoke; queued work rechecks credentials                                                     |
| POST `/v1/credentials/{id}/test`                     | Authentication/catalog check; no generation                                                  |
| GET `/v1/models?provider=horde&credential_id=`       | Verified models, selectable flags and reasons                                                |
| POST `/v1/generations`                               | Required Idempotency-Key, encrypted prompt + queued job + outbox transaction, 202 + Location |
| GET `/v1/generations?limit=24&cursor=`               | Stable owner-scoped UUID anchor pagination, limit 1–50                                       |
| GET `/v1/generations/{id}`                           | Current state, route reason and private artifact metadata                                    |
| DELETE `/v1/generations/{id}`                        | Tombstone terminal job and deny new downloads; pending deletion conflicts                    |
| GET `/v1/artifacts/{id}/download`                    | Two-minute signed relative content URL                                                       |
| GET `/v1/artifacts/{id}/content?expires=&signature=` | Signature **and owner authorization** required; stream private image                         |

Example request body:

```json
{
  "provider": "horde",
  "model_id": "Deliberate",
  "prompt": "A peaceful watercolor mountain landscape",
  "aspect_ratio": "1:1"
}
```

Use the actual discovered model ID; available workers vary. `credential_id` is optional for Horde and absent for local providers. Cloudflare requires its owned connection and square aspect ratio. Gemini returns `billing` without contacting a generation endpoint. Replaying the same owner/key/payload returns the same job; changed payload is 409. No replay resubmits a provider job.

States: `queued`, `submitting`, `running`, `succeeded`, `failed`, `reconciliation_required`. Unknown external acceptance and irrecoverable sync-result loss require operator investigation; M1 exposes no resubmit/reconciliation button. Polling of persisted async operations resumes after worker restart. Submit activities have one attempt; safe reads/polls and S3 same-object uploads have bounded retries.

JSON bodies are strict, limited to 16 KiB; unsupported content types return 415. Validation is 400, owner access 401, missing/foreign resources 404, conflicts 409, provider capability/auth/billing/policy errors 422, quota errors 429 and unavailable dependencies 500/503. Images are capped at 16 MiB and 32 million pixels, with MIME sniffing. Artifact retention is 30 days; expired/deleted records deny download before physical garbage collection.

The browser uses Next's same-origin `/api/flow/*` proxy and an eight-hour signed HttpOnly SameSite=Strict session created by `/api/session`. The owner token stays server-side after login; provider keys are sent only while saving, cleared afterwards and never stored in localStorage. Mutations require an exact Origin match. The Go API is loopback/private and independently checks Bearer authorization. This is a local single-owner MVP; production multi-user authorization is M4.
