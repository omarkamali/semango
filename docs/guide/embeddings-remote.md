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

## Manual Dimension Override (Truncation)

You can specify a custom dimension using the `dim` parameter.

```yaml
embedding:
  provider: openai
  model: text-embedding-3-large
  dim: 1024  # Truncate large embeddings to 1024
```

For OpenAI `text-embedding-3-*` models, Semango passes this value directly to the API's `dimensions` parameter, which is more efficient than manual truncation. For other models (including self-hosted compatibles), Semango truncates the vector returned by the endpoint to the first `dim` elements.

> [!IMPORTANT]
> **Matryoshka Support**: Truncation is most effective with models trained via **Matryoshka Representation Learning** (e.g., OpenAI's v3 models). For older models like `text-embedding-ada-002`, truncating the vector will likely lead to poor search performance. Check [sbert.net](https://sbert.net) for more information on compatible models.
