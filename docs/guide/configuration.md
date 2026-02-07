# Configuration

Semango reads `semango.yml`. You don’t need to write it by hand—`semango init` creates a fully-commented config with defaults.

```bash
semango init
```

## Recommended starting config (aligned with current loaders)

```yaml
embedding:
  provider: local
  model: onnx-models/bge-small-en-v1.5-onnx
  batch_size: 48
  concurrent: 4
  gpu: true
  model_cache_dir: "${SEMANGO_MODEL_DIR:=~/.cache/semango}"

lexical:
  enabled: true
  index_path: ./semango/index/bleve
  bm25_k1: 1.2
  bm25_b: 0.75

hybrid:
  vector_weight: 0.7
  lexical_weight: 0.3
  fusion: linear

files:
  include:
    - "**/*.{md,txt,go,js,ts,py,jsx,tsx}"
    - "**/*.pdf"
    - "**/*.csv"
    - "**/*.tsv"
    - "**/*.json"
    - "**/*.jsonl"
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
  chunk_size: 1000
  chunk_overlap: 200

server:
  host: 0.0.0.0
  port: 8181

ui:
  enabled: true

tabular:
  max_rows_embedded: 1000
  sampling: random
  min_text_tokens: 5
  delimiter: ""
```

`semango init` writes a more extensive template (including `reranker`, `plugins`, and `mcp`). Those sections are parsed but **not implemented** in the current codebase.

## Section-by-section reference

### `embedding`
- `provider`: `local` or `openai`.
- `model`: local ONNX model ID or OpenAI model name.
- `batch_size`, `concurrent`: throughput controls.
- `model_cache_dir`: where ONNX models are stored.
- `api_key`, `api_key_env`, `base_url`, `base_url_env`: used for `openai` provider.

If `provider` is omitted, the code defaults to `openai`.

### `lexical`
- `enabled`: enable BM25.
- `index_path`: Bleve index location.
- `bm25_k1`, `bm25_b`: BM25 tuning.

### `reranker`
Parsed but **not used** in the current query pipeline. Keep disabled unless you are modifying the code.

### `hybrid`
- `vector_weight`, `lexical_weight`: fusion weights.
- `fusion`: currently `linear` (RRF not implemented in code).

### `files`
Controls what gets indexed. The default `include` list is broader than what is currently supported by loaders (see [Ingestion](/guide/ingestion)).

### `server`
- `host`, `port`: HTTP binding.
- `auth`: parsed but **not enforced** in the current server.
- `tls_cert`, `tls_key`: parsed; TLS is not wired up in the current HTTP server.

### `plugins`
Parsed but **not implemented** in the current pipeline.

### `ui`
Toggle the embedded UI.

### `mcp`
Parsed but **not implemented** in the current build. See [MCP status](/guide/mcp).

### `tabular`
Controls CSV/JSON/JSONL ingestion.
- `max_rows_embedded`: cap per file.
- `sampling`: `random` or `stratified`.
- `min_text_tokens`: minimum tokens per row.
- `delimiter`: CSV delimiter (e.g., `"\t"` for TSV).

## Schema (secondary)

Semango validates configs using a CUE schema. This is for tooling and validation, not required reading.

- Download: [config.cue](/config.cue)
