# 🥭 Semango

[![CI](https://github.com/omarkamali/semango/actions/workflows/ci.yml/badge.svg)](https://github.com/omarkamali/semango/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Semango** is a hybrid search engine that combines lexical (BM25) and semantic (vector) search. Index your codebase, documentation, or knowledge base and search with natural language queries.

## Features

- **Hybrid Search**: Combines BM25 lexical search (via Bleve) with vector similarity search (via FAISS)
- **Multi-format Ingestion**: Markdown, code files, PDFs, images, and tabular data (CSV, JSON, Parquet, SQLite)
- **Embedding Providers**: OpenAI API or local ONNX models (e.g., all-MiniLM-L6-v2)
- **Web UI**: Embedded React-based search interface with dark mode
- **REST API**: Token-authenticated HTTP API for programmatic access
- **MCP Support**: Model Context Protocol integration for AI assistants
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

# 2. Set API tokens (for authentication)
export SEMANGO_TOKENS="your-secret-token"

# 3. Index your content
semango index

# 4. Start the server
semango server
```

Open http://localhost:8181 for the web UI, or query the API:

```bash
curl -X POST http://localhost:8181/search \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"query": "how does authentication work", "top_k": 5}'
```

## Configuration

Semango uses `semango.yml` for configuration. Key options:

```yaml
embedding:
  provider: openai          # or "local" for ONNX models
  model: text-embedding-3-large
  
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

| Variable | Description |
|----------|-------------|
| `SEMANGO_TOKENS` | Comma-separated list of valid API tokens |
| `OPENAI_API_KEY` | OpenAI API key (when using `provider: openai`) |
| `SEMANGO_ENV_FILE` | Path to `.env` file to load |
| `SEMANGO_MODEL_DIR` | Cache directory for local models |

## Documentation

- [Configuration Guide](docs/SEMANGO_GUIDE.md) - Full configuration reference
- [Local Embeddings](docs/LOCAL_EMBEDDER.md) - Using local ONNX models
- [Tabular Data](docs/tabular.md) - Ingesting CSV, JSON, Parquet files

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT License](LICENSE).

## Author

**Omar Kamali** - [semango@omarkama.li](mailto:semango@omarkama.li) | [X / Twitter](https://x.com/omarkamali)
