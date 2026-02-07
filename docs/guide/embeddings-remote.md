# Remote embeddings (OpenAI-compatible)

Semango’s `openai` provider uses the OpenAI Embeddings API and supports OpenAI‑compatible endpoints via `base_url`.

## Minimal config

```yaml
embedding:
  provider: openai
  model: text-embedding-3-large
  api_key_env: OPENAI_API_KEY
```

## OpenAI-compatible providers

If you use a compatible provider (self-hosted or third‑party), set `base_url` or `base_url_env`:

```yaml
embedding:
  provider: openai
  model: text-embedding-3-small
  api_key_env: MY_EMBEDDING_API_KEY
  base_url_env: MY_OPENAI_BASE_URL
```

Semango will pass requests to `base_url` using the OpenAI-compatible schema.

## Known model dimensions

Semango has a small list of known embedding dimensions (used for validation):

- `text-embedding-3-large` → 3072
- `text-embedding-3-small` → 1536
- `text-embedding-ada-002` → 1536
- `text-embedding-nomic-embed-text-v1.5` → 768

If you use a different model, Semango currently cannot infer its dimension and will error.
