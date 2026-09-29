# Gemini Developer API

Status: planned for M1; M0 performs no Google requests.

1. Open [Google AI Studio API keys](https://aistudio.google.com/app/apikey) and create a key for your permitted project.
2. Follow [Google's key guidance](https://ai.google.dev/gemini-api/docs/api-key); keep the key on the server and restrict it appropriately.
3. Review billing and [project quotas](https://ai.google.dev/gemini-api/docs/rate-limits). Availability depends on account/project/model.
4. Once OF-006/007 ship, connect through the protected Providers page and test access. Never put the key in a public environment variable.

Implementation must verify the current [image documentation](https://ai.google.dev/gemini-api/docs/image-generation) and [video operation documentation](https://ai.google.dev/gemini-api/docs/veo), using the official [Google Gen AI Go SDK](https://pkg.go.dev/google.golang.org/genai). The model catalog is dynamic; do not infer image output from a text model's generateContent method. Video operations require saved identifiers and durable polling. Prices/quotas are not hardcoded promises. Never cycle project keys to evade limits.

Official documentation reviewed on 2026-09-29; recheck before implementing the adapter.
