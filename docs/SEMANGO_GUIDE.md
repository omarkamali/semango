# Semango Guide

This guide gives you a progressive path from a minimal setup to a fully featured configuration. It covers:

- Quickstart (5 minutes)
- Configuration reference (what each key does)
- Operating Semango (indexing, serving, auth, env expansion, logs)
- Advanced usage (hybrid, reranker, tabular, plugins, UI, MCP)
- Performance & scaling
- Troubleshooting & FAQs

Refer to supporting docs:
- docs/LOCAL_EMBEDDER.md — details on running local embeddings
- docs/tabular.md — tabular ingestion concepts and examples

---

## Quickstart

1) Create a minimal `semango.yml` in your project root:

```yaml
embedding:
  provider: local
  local_model_path: "onnx-models/all-MiniLM-L6-v2-onnx"
  batch_size: 48
  concurrent: 4
  model_cache_dir: ${SEMANGO_MODEL_DIR:=~/.cache/semango}
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
    - '**/*.md'
    - '**/*.go'
    - '**/*.{png,jpg,jpeg}'
    - '**/*.pdf'
    - '**/*.csv'
    - '**/*.json'
    - '**/*.jsonl'
    - '**/*.parquet'
  exclude:
    - .git/**
    - node_modules/**
    - vendor/**
  chunk_size: 1000
  chunk_overlap: 200
server:
  host: 0.0.0.0
  port: 8181
  auth:
    type: token
    token_env: SEMANGO_TOKENS
  tls_cert: ""
  tls_key: ""
ui:
  enabled: true
mcp:
  enabled: true
# Optional: structured data (CSV/JSON/Parquet/SQLite)
tabular:
  max_rows_embedded: 50000
  sampling: random
  min_text_tokens: 5
  # delimiter: "\t"  # for TSV
```

2) Prepare tokens (for the HTTP API):

```bash
export SEMANGO_TOKENS="devtoken123,another-token"
```

3) Initialize and index:

```bash
semango init
semango index
```

4) Run the server:

```bash
semango
# or, if you prefer explicit
# semango serve
```

5) Query the API:

```bash
curl -s -H "Authorization: Bearer devtoken123" \
  -X POST http://localhost:8181/search \
  -d '{"query": "vector databases in our README"}' | jq .
```

---

## Configuration Reference

Semango validates config against a CUE schema (see `docs/config.cue`). Top-level keys:

- `embedding` (provider, model, local_model_path, batch_size, concurrent, model_cache_dir)
  - provider: "local" | "openai" | "cohere" | "voyage"
  - model: string (required for hosted providers)
  - local_model_path: path for local models
  - batch_size: int (1..512), default 48
  - concurrent: int (>=1), default 4
  - model_cache_dir: path (supports env/default expansion)

- `lexical` (BM25 & index path)
  - enabled: bool, default true
  - index_path: path for Bleve index
  - bm25_k1: float, default 1.2
  - bm25_b: float, default 0.75

- `reranker`
  - enabled: bool, default false
  - provider: "cohere" | "openai" | "local" (default cohere)
  - model: string (default rerank-english-v3.0)
  - batch_size: int (>=1), default 32
  - per_request_override: bool, default true

- `hybrid`
  - vector_weight: 0.0..1.0, default 0.7
  - lexical_weight: 0.0..1.0, default 0.3
  - fusion: "linear" | "rrf"

- `files`
  - include: glob list for files to ingest
  - exclude: glob list for files/folders to skip
  - chunk_size: int, default 1000
  - chunk_overlap: int, default 200

- `server`
  - host: string, default 0.0.0.0
  - port: int (1..65535), default 8181
  - auth:
    - type: "token"
    - token_env: env var holding a comma-separated token list, default SEMANGO_TOKENS
  - tls_cert: optional
  - tls_key: optional

- `ui`
  - enabled: bool, default true

- `mcp`
  - enabled: bool, default true

- `tabular` (for CSV/TSV/JSON/JSONL/Parquet/SQLite)
  - max_rows_embedded: int >= 1, default 50000
  - sampling: "random" | "stratified"
  - min_text_tokens: int >= 1, default 5
  - delimiter: string (e.g., "," or "\t"); for CSV/TSV readers

Notes on environment expansion:
- Values like `${VAR:=default}` expand to `$VAR` if set, else `default` (with `~` expansion).
- Plain `$VAR` or `${VAR}` expand to the environment variable if present.
- `~` at start of a path expands to the current user’s home directory.

---

## Operating Semango

- Initialize a new index:
  ```bash
  semango init
  ```

- Index documents according to `files.include`/`exclude`:
  ```bash
  semango index
  ```

- Re-index after changing configuration or content:
  ```bash
  rm -rf semango/
  semango init
  semango index
  ```

- Start the server (HTTP API + optional UI):
  ```bash
  semango
  # or
  # semango serve
  ```

- Authentication:
  - Set tokens in the environment variable configured by `server.auth.token_env` (default `SEMANGO_TOKENS`).
  - Send `Authorization: Bearer <token>` on requests.

- Logs:
  - Logs are printed to stdout/stderr in JSON. Look for `level`, `msg`, and `error_message`.

---

## Build & Commands

This section summarizes common ways to build, run, and develop Semango.

### Makefile targets

The repository includes a `Makefile` with helpful targets:

- `make ui-build`
  - Installs UI deps and builds the React app in `ui/`.
- `make ui-copy`
  - Copies the built UI from `ui/dist/` into `internal/api/ui/` for embedding.
- `make build`
  - Runs `ui-build` and `ui-copy`, then builds the Go binary with CGO flags. Produces `./semango`.
- `make build-no-ui`
  - Builds only the Go binary (no UI embedding). Useful during backend development.
- `make run`
  - Builds and runs `./semango`.
- `make dev-ui`
  - Starts the Vite dev server for the UI (`yarn dev` in `ui/`).
- `make dev-server`
  - Builds the Go binary without UI and runs it (useful alongside `make dev-ui`).
- `make test`
  - Runs `go test ./...` with the proper CGO flags.
- `make ui-clean` / `make clean`
  - Cleans generated artifacts and node_modules.

Notes:
- These targets rely on CGO flags to link FAISS and ONNX Runtime from `libs/`. See Direct Go build below if you build without the Makefile.

### Direct Go build (without Makefile)

The Makefile sets `CGO_LDFLAGS` to ensure the FAISS C API and ONNX Runtime shared libraries are found at runtime:

```bash
export CGO_LDFLAGS='-L/app/libs -lfaiss_c -L/app/libs -lonnxruntime -Wl,-rpath,/app/libs'
go build -o semango ./cmd/semango
```

If your working directory differs, adjust the `-L` paths to your local `libs/` folder. The `-Wl,-rpath,/app/libs` ensures the loader can find the `.so` files at runtime without additional `LD_LIBRARY_PATH` configuration.

Install scripts:
- `./install_faiss.sh`
- `./install_onnxruntime.sh`

These scripts can help fetch or prepare the native dependencies if you are setting up from scratch.

### UI-only workflow

When iterating on the UI, run the backend without embedding the UI and use Vite for hot reloading:

```bash
# terminal 1: backend
make dev-server

# terminal 2: UI dev server
make dev-ui
```

Point your browser to the Vite dev server URL (usually `http://localhost:5173`). The API will be available from the backend at the configured port (default `8181`).

### Docker

This repo includes a `Dockerfile` suitable for building a production image with the embedded UI. Example:

```bash
docker build -t semango:latest .
docker run --rm -p 8181:8181 \
  -e SEMANGO_TOKENS="devtoken123" \
  -v "$PWD/semango.yml":/app/semango.yml \
  semango:latest
```

If you prefer using a `.env` file, either bake it into the image or mount it and set `SEMANGO_ENV_FILE`:

```bash
docker run --rm -p 8181:8181 \
  -e SEMANGO_ENV_FILE=/run/secrets/semango.env \
  -v "$PWD/.env":/run/secrets/semango.env:ro \
  -v "$PWD/semango.yml":/app/semango.yml \
  semango:latest
```

### Docker Compose

A `docker-compose.yml` is provided as a starting point. Typical usage:

```bash
docker compose up --build
```

Customize volumes and environment variables in `docker-compose.yml` to point to your data, config, and env files.

### Testing

Run all Go tests with the correct CGO flags:

```bash
make test
# or
CGO_LDFLAGS='-L./libs -lfaiss_c -L./libs -lonnxruntime -Wl,-rpath,./libs' go test ./...
```

For UI tests, add your preferred JS test runner (e.g., Vitest/Jest) in `ui/` and run via `yarn test`.


## Advanced Usage

- Hybrid search
  - Adjust `hybrid.vector_weight` and `hybrid.lexical_weight` to balance vectors vs BM25.
  - Switch `hybrid.fusion` to `rrf` for Reciprocal Rank Fusion in some scenarios.

- Reranker
  - Enable `reranker.enabled: true` and set `provider/model` for better final ranking.
  - Control throughput with `reranker.batch_size`.

- Tabular ingestion
  - Include structured formats in `files.include` (csv, tsv, json, jsonl, parquet, sqlite).
  - Tune `tabular.max_rows_embedded` and `tabular.sampling` to control vector counts.
  - Use `tabular.min_text_tokens` to skip near-empty rows.
  - See `docs/tabular.md` for how rows are transformed and example API queries.

- Plugins
  - Add shared objects or plugin paths under `plugins:`.
  - Example:
    ```yaml
    plugins:
      - plugins/
      - ../shared/my_custom.so
    ```

- UI
  - `ui.enabled: true` exposes a simple web UI when the server runs.

- MCP
  - `mcp.enabled: true` integrates with Model Context Protocol clients.

- Local embedder
  - See `docs/LOCAL_EMBEDDER.md` for model selection and migration from OpenAI.

---

## Performance & Scaling

- Embedding throughput
  - `embedding.batch_size`: increase for higher GPU/CPU utilization until latency/oom is unacceptable.
  - `embedding.concurrent`: number of concurrent workers producing embeddings.

- Index size and speed
  - `lexical.index_path`: set to a fast disk; for large corpora, consider SSD/NVMe.
  - `files.chunk_size` / `files.chunk_overlap`: larger chunks reduce vector count but may hurt recall.

- Tabular controls
  - `tabular.max_rows_embedded`: hard cap per file; keep within budget.
  - `tabular.sampling`: choose `stratified` for more uniform coverage when data is skewed.

- Runtime parallelism
  - Ensure system has adequate CPU threads and memory; adjust OS limits if needed.

---

## Troubleshooting & FAQs

- Unknown field in configuration (Exit 78)
  - Cause: mismatch between your YAML and the CUE schema.
  - Fix: ensure fields exist per `docs/config.cue`. For example, `files.chunk_size` and `files.chunk_overlap` are valid and should be present in the schema. Update the schema if you vendor or embed it elsewhere.

- Failed to unify CUE #Config definition: `tabular.max_rows_embedded`
  - Cause: missing `tabular` section or invalid value (must be int >= 1).
  - Fix: add:
    ```yaml
    tabular:
      max_rows_embedded: 1000
      sampling: random
      min_text_tokens: 5
    ```

- My tokens are not recognized
  - Ensure `export SEMANGO_TOKENS="token1,token2"` is set in the environment before running the server.

- Local model not found
  - Check `embedding.local_model_path` and that the path exists; see `docs/LOCAL_EMBEDDER.md` for supported models.

- Slow indexing
  - Increase `embedding.batch_size` carefully; check disk IO and CPU utilization.

---

## Appendix: Example full configuration

```yaml
embedding:
  provider: local
  local_model_path: "onnx-models/all-MiniLM-L6-v2-onnx"
  batch_size: 64
  concurrent: 4
  model_cache_dir: ${SEMANGO_MODEL_DIR:=~/.cache/semango}
lexical:
  enabled: true
  index_path: ./semango/index/bleve
  bm25_k1: 1.2
  bm25_b: 0.75
reranker:
  enabled: true
  provider: cohere
  model: rerank-english-v3.0
  batch_size: 32
  per_request_override: true
hybrid:
  vector_weight: 0.65
  lexical_weight: 0.35
  fusion: rrf
files:
  include:
    - '**/*.md'
    - '**/*.go'
    - '**/*.pdf'
    - '**/*.csv'
    - '**/*.tsv'
    - '**/*.json'
    - '**/*.jsonl'
    - '**/*.parquet'
    - '**/*.sqlite'
  exclude:
    - .git/**
    - node_modules/**
    - vendor/**
  chunk_size: 1000
  chunk_overlap: 200
server:
  host: 0.0.0.0
  port: 8181
  auth:
    type: token
    token_env: SEMANGO_TOKENS
  tls_cert: ""
  tls_key: ""
ui:
  enabled: true
mcp:
  enabled: true
tabular:
  max_rows_embedded: 50000
  sampling: stratified
  min_text_tokens: 5
  delimiter: ","
plugins:
  - plugins/
  - ../shared/my_custom.so
```
