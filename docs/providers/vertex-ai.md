# Vertex AI

Status: planned for M3, separate from Gemini Developer API.

Use [Vertex AI's Gen AI quickstart](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/start/quickstart). Configure a Google Cloud project, billing, Vertex API, permitted IAM roles and model location. Local development uses Application Default Credentials via `gcloud auth application-default login`. Deployment should use workload identity with least privilege. Do not upload service-account JSON through the web UI.

The common adapter contract remains unchanged; credentials and project/region configuration differ. Verify regional model availability and official SDK authentication at implementation time.
