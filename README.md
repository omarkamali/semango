# 🥭 Semango

[![CI](https://github.com/omarkamali/semango/actions/workflows/ci.yml/badge.svg)](https://github.com/omarkamali/semango/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**🥭 Semango** is a hybrid search engine that combines lexical (BM25) and semantic (vector) search. Index your codebase and docs and search with natural language queries.

Website & docs: https://semango.org

## Features

- **Hybrid Search**: Combines BM25 lexical search (Bleve) with vector similarity (FAISS)
- **Multi-format Ingestion**: Markdown/text, code (plain text), PDFs, CSV/JSON/JSONL, Parquet, SQLite, Excel
- **Incremental Indexing**: Only re-indexes files that have changed (by mtime + size), making subsequent runs near-instant
- **Embedding Providers**: OpenAI-compatible API or local ONNX models
- **GPU Acceleration**: Built-in support for CUDA-accelerated local embeddings
- **MCP Server**: Model Context Protocol support for LLM tool-use (stdio and SSE transports)
- **Web UI**: Embedded React-based search interface with dark mode
- **REST API**: Lightweight HTTP API for programmatic access
- **Graceful Shutdown**: Clean Ctrl+C handling across all commands
- **Single Binary**: Self-contained executable with embedded UI assets

## Installation

### Download Binary

```bash
# macOS / Linux
curl -L "https://github.com/omarkamali/semango/releases/latest/download/semango_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz" | tar xz
sudo mv semango /usr/local/bin/
```

### Docker

```bash
docker pull ghcr.io/omarkamali/semango:latest
docker run -p 8181:8181 -v $(pwd):/data ghcr.io/omarkamali/semango:latest
```

### Build from Source

Requires Go 1.23+, Node.js 20+, and CGO dependencies (FAISS, OpenBLAS).

```bash
git clone https://github.com/omarkamali/semango.git
cd semango
make build
```

## Quick Start

```bash
# 1. Initialize configuration
semango init

# 2. Index your content
semango index

# 3. Start the server (includes web UI + auto-reconciliation)
semango server
```

Open http://localhost:8181 for the web UI, or query the API:

```bash
curl -X POST http://localhost:8181/search \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"query": "how does authentication work", "top_k": 5}'
```

### CLI Commands

| Command | Description |
|---------|-------------|
| `semango init` | Create a default `semango.yml` config file |
| `semango index` | Index files (incremental — skips unchanged files) |
| `semango index stats` | Show indexing statistics (document counts) |
| `semango search <query>` | Search from the command line |
| `semango server` | Start the HTTP server with UI and periodic reconciliation |
| `semango mcp stdio` | Start an MCP server over stdin/stdout |
| `semango mcp sse` | Start an MCP server over HTTP/SSE |
| `semango models list` | List available local ONNX models |
| `semango version` | Print version information |

> **Tip:** Running `semango index` a second time without file changes completes instantly — only modified files are re-embedded and re-indexed.
```

## Configuration

Semango uses `semango.yml` for configuration. Key options:

```yaml
embedding:
  provider: openai          # or "local" for ONNX models
  model: text-embedding-3-large
  # Optional: base_url, api_key, api_key_env, base_url_env
  
lexical:
  enabled: true
  index_path: ./semango/index/bleve
  
hybrid:
  vector_weight: 0.7
  lexical_weight: 0.3
  fusion: linear            # or "rrf"
  
files:
  include:
    - '**/*.md'
    - '**/*.go'
    - '**/*.pdf'
  exclude:
    - .git/**
    - node_modules/**

server:
  port: 8181
  auth:
    type: token
    token_env: SEMANGO_TOKENS
```

See [docs/SEMANGO_GUIDE.md](docs/SEMANGO_GUIDE.md) for the complete configuration reference.

## Environment Variables

| Variable | Description | Default / Optional |
|----------|-------------|--------------------|
| `SEMANGO_TOKENS` | Comma-separated list of valid API tokens | Parsed but not enforced yet |
| `OPENAI_API_KEY` | API key to use when `provider: openai` | Used if `api_key` or `api_key_env` not set |
| `OPENAI_BASE_URL` | Base URL for OpenAI-compatible APIs | Defaults to OpenAI official endpoint |
| `SEMANGO_ENV_FILE` | Path to `.env` file to load | Defaults to `.env` |
| `SEMANGO_MODEL_DIR` | Cache directory for local ONNX models | Defaults to `~/.cache/semango` |

You can also customize which environment variables Semango looks for by setting `api_key_env` or `base_url_env` in your `semango.yml`.

## MCP (Model Context Protocol)

Semango can act as an [MCP](https://modelcontextprotocol.io/) server, letting LLMs use your indexed content as a search tool:

```bash
# stdio transport (for Claude Desktop, etc.)
semango mcp stdio --config semango.yml

# SSE transport (HTTP-based, for remote clients)
semango mcp sse --host 0.0.0.0 --port 8080 --config semango.yml
```

## Documentation

- https://semango.org/guide/
- [Local Embeddings](docs/LOCAL_EMBEDDER.md) — Using local ONNX models
- [Tabular Data](docs/tabular.md) — Ingesting CSV/JSON/JSONL/Parquet/SQLite/Excel files
- [MCP Integration](docs/guide/mcp.md) — Using Semango with LLMs

Online: https://semango.org

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT License](LICENSE).

## Author

Built by **Omar Kamali** (https://omarkamali.com) — Omneity Labs (https://omneitylabs.com)
