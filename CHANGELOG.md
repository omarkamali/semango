# Changelog

All notable changes to Semango will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial public release preparation
- GitHub Actions CI/CD workflows
- GoReleaser configuration for cross-platform builds
- Production Dockerfile for containerized deployment

## [0.1.0] - 2024-12-13

### Added
- **Hybrid Search Engine**: Combined lexical (BM25 via Bleve) and semantic (FAISS vector) search
- **Multi-format Ingestion**: Support for Markdown, Go, PDF, images, and code files
- **Tabular Data Support**: CSV, TSV, JSON, JSONL, Parquet, and SQLite ingestion
- **Embedding Providers**: OpenAI and local ONNX model support
- **HTTP API**: RESTful search API with token-based authentication
- **Embedded Web UI**: React-based search interface with dark mode
- **MCP Support**: Model Context Protocol integration
- **Configuration**: YAML-based configuration with CUE schema validation
- **Environment Expansion**: Support for `${VAR:=default}` and `~` in config paths
- **CLI Commands**: `init`, `index`, `server` commands

### Technical
- Go 1.23 with CGO for FAISS and ONNX bindings
- Bleve 2.4 for full-text search
- FAISS 1.8.0 for vector similarity search
- React 18 / Vite / Tailwind CSS for UI
- Cobra for CLI framework

---

[Unreleased]: https://github.com/omarkamali/semango/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/omarkamali/semango/releases/tag/v0.1.0
