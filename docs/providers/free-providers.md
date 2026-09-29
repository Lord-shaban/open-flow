# Free image providers

Verified against official sources on **2026-09-29**. M1 was changed from Gemini-first to free-first at the user's request. Prices and model availability can change; discovery verifies IDs and service availability, not billing plans.

| Provider              | Account/card                               | Free allowance                                                                               | Open Flow behavior                                                                                               |
| --------------------- | ------------------------------------------ | -------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| AI Horde              | Anonymous access: neither required         | Volunteer compute; queue/worker availability, kudos priority rather than a fixed image quota | Default; live worker-backed model discovery, async submit/poll, inline image transport                           |
| Cloudflare Workers AI | Free account + API token; no card required | 10,000 neurons/day, account-wide; reset 00:00 UTC                                            | Optional explicitly confirmed Workers Free account, verified FLUX.1 Schnell, square images, no automatic upgrade |
| ComfyUI               | Local server; no hosted account/card       | Your hardware capacity                                                                       | Optional local checkpoint discovery and a fixed compatible image workflow                                        |
| Training canvas       | Neither required                           | Local machine capacity                                                                       | Procedural renderer for system-design exercises; **not AI**                                                      |
| Gemini image API      | Paid billing for current image models      | No free image tier for the enabled image IDs                                                 | Generation blocked at admission and worker; authenticated discovery/testing only                                 |

## AI Horde

[AI Horde](https://www.aihorde.net/) is a free community service. The [official integration guide](https://github.com/Haidra-Org/AI-Horde/blob/main/README_integration.md) permits anonymous access with `0000000000`. Open Flow queries available image workers and permits only currently discovered image models. Anonymous priority can be low and queue times vary. A registered, encrypted key is optional; it is never rotated to bypass limits.

**Anonymous generations may be shared publicly**, including prompt/image metadata. Private Open Flow storage does not make an anonymous provider submission private. The composer discloses this before submission. Use an appropriate registered account and review Horde policy, or choose local inference for private material. Open Flow requests `shared:false`, which cannot override Horde's anonymous sharing rules.

Transport uses `r2:false`, accepts inline PNG/JPEG/WebP only, enforces 16 MiB and decoded-dimension limits, and rejects returned remote URLs. Text prompts only, one image, 20 steps, bounded aspect ratios and trusted workers. No arbitrary workflows or provider URLs come from the browser. See [image API documentation](https://aihorde.net/docs/image/).

Live verification: one anonymous 512×512 Deliberate request completed on 2026-09-29 and returned a valid WebP without account/card/key setup. This proves that path at the time of testing, not a queue-time or availability guarantee. CI uses HTTP fixtures rather than consuming community compute.

## Cloudflare Workers AI

Cloudflare advertises [free signup without a credit card](https://www.cloudflare.com/products/workers-ai/). Its [pricing documentation](https://developers.cloudflare.com/workers-ai/platform/pricing/) gives Workers Free 10,000 neurons/day and states that requests fail after the allowance is exhausted unless the user upgrades. On Workers Paid, usage above the allocation incurs charges. Do not connect a paid account when following this project's no-card requirement.

1. Create a **Workers Free** account without a payment method.
2. Follow [REST API setup](https://developers.cloudflare.com/workers-ai/get-started/rest-api/), create a scoped Workers AI token and copy the 32-character account ID.
3. Open Connections, select Cloudflare, enter both fields and confirm the account is Free without a payment method. Save encrypted and test discovery; this does not generate an image.
4. Select Cloudflare and the discovered [FLUX.1 Schnell](https://developers.cloudflare.com/workers-ai/models/flux-1-schnell/) model. M1 uses four steps and square output. Free allowance is measured in compute units, **not a promised number of images**.

The confirmation is a user assertion, not a provider billing API verification. Open Flow cannot prevent provider charges if someone supplies an already paid account while falsely confirming Free. There is no automatic plan upgrade, AI Gateway prepaid credit use, failover, or credential cycling. A quota failure becomes a visible failed job. The adapter has fixture coverage; no real Cloudflare image test was run without a user-supplied account token.

## Other researched options

[Hugging Face Inference Providers](https://huggingface.co/docs/inference-providers/pricing) gives free users $0.10 monthly credit; this is too small to promise a dependable image allowance and is not M1's default. Other metered services such as Pollinations need their current credit rules verified before inclusion. Paid fal/Replicate/Vertex integrations are not activated. Self-hosted PostgreSQL, Kafka, Temporal and [SeaweedFS](https://github.com/seaweedfs/seaweedfs) require no hosted service/card; running them still uses your own machine's resources.
