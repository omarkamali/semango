# Semango 1.0 Implementation Roadmap

> **WARNING:** Previous roadmap status was misleading. Only text file indexing/search, config, CLI, and Bleve are truly implemented. All other features are pending and not implemented. This roadmap now reflects the real status.

This roadmap breaks down the implementation of Semango into logical phases, prioritizing components to deliver incremental value while managing dependencies between systems.

## Phase 1: Core Infrastructure & Minimal Viable Indexer (2-3 weeks)

### Goals
- Establish project structure and development environment
- Implement core configuration, logging, and CLI
- Create basic text indexing with a simplified pipeline

### Tasks
1. **Project Setup** (P0)
   - [x] Repository structure following the spec layout
   - [x] Go module setup with initial dependencies
   - [x] Makefile/build scripts and basic CI (Makefile done, CI pending)
   - [x] Development container for consistent environments (Done)

2. **Core Configuration System** (P0)
   - [x] `semango.yml` parser
   - [x] CUE schema validation implementation
   - [x] Environment variable handling (basic done, advanced pending)
   - [x] Basic CLI structure with cobra

3. **Basic Logging & Metrics** (P0)
   - [x] Structured logging setup (slog)
   - [x] Error reporting framework (basic done, advanced (stack traces & context) done)
   - [ ] Metrics collection (Pending; no real metrics, only stub)

4. **Minimal File Processing** (P0)
   - [x] Filesystem crawler (filepath.WalkDir + doublestar, basic include/exclude)
   - [x] Text file loader implementation (Done)
   - [x] In-memory representation storage (Done)

5. **Bleve Integration** (P1)
   - [x] Basic Bleve index setup
   - [x] Simple indexing pipeline for text
   - [x] Initial search query implementation

### Deliverables
- [x] Working `semango init` command
- [x] `semango index` for simple text files
- [x] Basic search capability for text content
- [x] Initial test suite covering core functionality

---

## Phase 2: Vector Search & Advanced Loaders (3-4 weeks)

### Goals
- Implement vector search capabilities
- Add support for code, PDF, and image parsing
- Create a unified indexing pipeline

### Tasks
1. **FAISS/Vector Search Integration** (P0)
   - [x] CGO bindings setup (Completed - faiss_c.go with build constraints)
   - [x] Vector index creation and management (Completed - FaissIndex struct with CRUD operations)
   - [x] Cross-platform build considerations (Completed - Linux support via Docker, build constraints for platform-specific CGO)

2. **OpenAI Embedding Provider** (P0)
   - [x] Implement the Embedder interface (Completed - defined in pkg/semango/interfaces.go)
   - [x] Batch processing with rate limiting (Completed - OpenAIEmbedder with configurable batching and golang.org/x/time/rate)
   - [x] Error handling and retries (Completed - exponential backoff retry logic with 3 attempts)

3. **Advanced Loaders** (P1)
   - [ ] Code loader with Tree-sitter (Pending)
   - [ ] PDF text extraction (Pending)
   - [ ] Image processing pipeline (Pending)

4. **Unified Indexing Pipeline** (P0)
   - [ ] Representation manager implementation (Pending)
   - [ ] Parallel processing framework (Pending)
   - [ ] BoltDB metadata integration (Pending)

5. **Chunking & Document Processing** (P1)
   - [ ] Text chunking strategies (Pending)
   - [ ] Metadata extraction (Pending)
   - [ ] Document boundaries management (Pending)

### Deliverables
- [x] Vector search capability (Completed - FAISS integration with load/save/search operations)
- [ ] Support for code files with syntax awareness (Pending)
- [ ] PDF indexing with text extraction (Pending)
- [ ] Complete indexing pipeline with filesystem watcher (Pending)
- [ ] Initial benchmark results for indexing performance (Pending)

---

## Phase 3: API & Query Pipeline (2-3 weeks)

### Goals
- Implement REST and gRPC APIs
- Build hybrid search capabilities
- Integrate reranking functionality

### Tasks
1. **REST API Implementation** (P0)
   - Gin HTTP server setup
   - Search endpoint implementation
   - Authentication middleware

2. **Hybrid Search** (P0)
   - Vector + BM25 fusion algorithms
   - Scoring normalization
   - Results aggregation

3. **Search Query Pipeline** (P1)
   - Query parsing
   - Filter implementation
   - Pagination and sorting

4. **Reranker Integration** (P2)
   - Reranker interface implementation
   - Cohere and local reranker support
   - Results post-processing

5. **gRPC & MCP Support** (P1)
   - Proto definitions
   - gRPC service implementation
   - Model Context Protocol support

### Deliverables
- [ ] Functional REST API with documented endpoints
- [ ] Working hybrid search (vector + lexical)
- [ ] Integrated reranking for improved results
- [ ] gRPC API for programmatic access
- [ ] API test suite and performance benchmarks

---

## Phase 4: UI & Additional Embedding Providers (2-3 weeks)

### Goals
- Build the React UI
- Add support for additional embedding providers
- Implement image and audio search capabilities

### Tasks
1. **React UI Implementation** (P0)
   - Setup Vite build pipeline
   - Implement core search UI
   - Results display with highlighting

2. **Additional Embedding Providers** (P1)
   - Cohere integration
   - Voyage integration
   - Local llama.cpp provider

3. **Media Search Capabilities** (P2)
   - CLIP integration for image search
   - Whisper integration for audio
   - Cross-modal search capability

4. **UI Refinements** (P1)
   - Dark mode support
   - Results filtering
   - Search history and saved searches

5. **Asset Bundling** (P1)
   - Go:embed integration
   - Static assets management
   - UI optimization

### Deliverables
- [ ] Functional web UI for search
- [ ] Support for all specified embedding providers
- [ ] Image and audio search capabilities
- [ ] Responsive design with dark mode
- [ ] UI tests and browser compatibility validation

---

## Phase 5: Plugin System & Advanced Features (2-3 weeks)

### Goals
- Implement the plugin system
- Add support for advanced features
- Enhance operational capabilities

### Tasks
1. **Plugin System** (P0)
   - Go plugin loading framework
   - Sandbox implementation
   - Plugin registration and discovery

2. **OCR Integration** (P1)
   - Tesseract binding via gosseract
   - PDF OCR pipeline
   - Language detection

3. **Plugin Examples** (P1)
   - Sample ipynb plugin
   - Plugin documentation
   - Testing framework

4. **Advanced Search Features** (P2)
   - Semantic filtering
   - Faceted search
   - Query expansion

5. **Performance Optimizations** (P1)
   - Caching layer
   - Index optimization
   - Query performance tuning

### Deliverables
- [ ] Working plugin system with documentation
- [ ] OCR support for scanned documents
- [ ] Sample plugins demonstrating extensibility
- [ ] Advanced search capabilities
- [ ] Performance tuning report and benchmarks

---

## Phase 6: Packaging & Production Readiness (2-3 weeks)

### Goals
- Prepare for production deployment
- Create distribution packages
- Complete documentation and tests

### Tasks
1. **Goreleaser Setup** (P0)
   - Cross-platform build configuration
   - Binary signing for macOS and Windows
   - Release automation

2. **Container Integration** (P1)
   - Docker/Podman image creation
   - docker-compose setup
   - Volume mounting and persistence

3. **Helm Chart** (P2)
   - Kubernetes deployment
   - Configuration options
   - Resource management

4. **Documentation** (P0)
   - User guide
   - API documentation
   - Deployment scenarios
   - Performance tuning guide

5. **Final Testing** (P0)
   - Integration test suite
   - Performance benchmarks
   - Security review
   - Cross-platform validation

### Deliverables
- [ ] Release binaries for all platforms
- [ ] Container images
- [ ] Helm chart for Kubernetes
- [ ] Comprehensive documentation
- [ ] Full test coverage report

---

## Implementation Guidelines

### Priority Levels
- **P0**: Critical path, must be completed before dependent items
- **P1**: Important for core functionality, should be addressed early
- **P2**: Enhances functionality but not blocking
- **P3**: Nice to have, can be deferred if necessary

### Development Practices
1. **Testing**
   - Unit tests for all packages
   - Integration tests for key workflows
   - Benchmark tests for performance-critical paths

2. **Code Reviews**
   - All PRs require review
   - Performance impact considered for critical paths
   - Security review for network-facing components

3. **Documentation**
   - Godoc for all exported functions
   - README updates with each phase
   - Examples for key functionality

4. **Versioning**
   - Follow semantic versioning
   - Tag pre-releases during development (v0.x)
   - Maintain changelog

### Technical Debt Management
- Schedule regular refactoring sessions
- Address TODOs before moving to the next phase
- Performance profiling at each phase boundary

## Estimated Timeline

| Phase | Duration | Cumulative |
|-------|----------|------------|
| 1: Core Infrastructure | 2-3 weeks | 2-3 weeks |
| 2: Vector Search & Loaders | 3-4 weeks | 5-7 weeks |
| 3: API & Query Pipeline | 2-3 weeks | 7-10 weeks |
| 4: UI & Embedding Providers | 2-3 weeks | 9-13 weeks |
| 5: Plugin System | 2-3 weeks | 11-16 weeks |
| 6: Packaging & Production | 2-3 weeks | 13-19 weeks |

**Total estimated time: 13-19 weeks (3-4.5 months)**

> Note: This timeline assumes 1-2 full-time engineers with Go experience and familiarity with ML/search concepts. Additional resources may accelerate delivery, particularly for parallel tracks like UI development. 