# Changelog

All notable changes to Semango will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-02-15

### Added
- Added support for custom dimensions to either override Semango's guess or truncate the provided embeddings.

## [0.2.0] - 2026-02-13

### Fixed

- Resolved UI layout artifacts (double mangos)
- Fixed FAISS support in Windows CI builds
- Resolved stale UI assets issue in CI and production binaries
- Fixed UTF-8 text mangling in indexing pipeline
### Changed
- **Unified Model Configuration**: Merged `local_model_path` into the `model` property for both `embedding` and `reranker`.
- **Hugging Face Mirroring**: Local model downloads now preserve their repository directory structure (e.g., `author/repo/`) and can automatically pull from Huggingface.
- **Provider Simplification**: Standardized on `openai` and `local` providers. Removed legacy `cohere` and `voyage` providers in favor of OpenAI-compatible endpoints.
- **Flexible Environment Variables**: Added `api_key_env` and `base_url_env` to customize which environment variables are used for API secrets and endpoints (defaults to `OPENAI_API_KEY` and `OPENAI_BASE_URL`).

### Added
- Added per-page timeouts for PDF extraction
- Added graceful shutdown handling for server and crawler
- Implemented incremental indexing with FingerprintStore
- Added version and commit metadata to UI footer
- Added indexing progress reporting to the UI
- Initial public release preparation
- GitHub Actions CI/CD workflows
- GoReleaser configuration for cross-platform builds
- Production Dockerfile for containerized deployment

## [0.1.0] - 2025-12-13

### Added
- **Hybrid Search Engine**: Combined lexical (BM25 via Bleve) and semantic (FAISS vector) search
- **Multi-format Ingestion**: Markdown/text, code (plain text), PDFs, CSV/JSON/JSONL
- **Embedding Providers**: OpenAI and local ONNX model support
- **HTTP API**: RESTful search API
- **Embedded Web UI**: React-based search interface with dark mode
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
[0.2.0]: https://github.com/omarkamali/semango/releases/tag/v0.2.0
[0.2.1]: https://github.com/omarkamali/semango/releases/tag/v0.2.1