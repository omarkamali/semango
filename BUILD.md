# Building Semango

This document describes how to build Semango with its embedded React UI.

**Author:** Omar Kamali <semango@omarkama.li>

## Prerequisites

### Required Software

| Software | Minimum Version | Notes |
|----------|-----------------|-------|
| Go | 1.23+ | CGO must be enabled |
| Node.js | 20+ | For UI build |
| Yarn | 1.22+ | Package manager |
| GCC/Clang | - | C compiler for CGO |
| CMake | 3.20+ | For building FAISS |

### System Libraries

#### macOS (Homebrew)
```bash
brew install openblas cmake
# FAISS must be built from source (see install_faiss.sh)
```

#### Ubuntu/Debian
```bash
sudo apt-get update
sudo apt-get install -y \
    build-essential \
    cmake \
    libopenblas-dev \
    libgflags-dev \
    tesseract-ocr \
    libleptonica-dev
```

#### FAISS Installation
See `install_faiss.sh` for building FAISS with C API support:
```bash
./install_faiss.sh
```

#### ONNX Runtime (for local embeddings)
See `install_onnxruntime.sh` for installation:
```bash
./install_onnxruntime.sh
```

## Quick Build

### Using Make (Recommended)

```bash
# Full build with UI embedding
make build

# Build without UI (development)
make build-no-ui

# Clean all build artifacts
make clean

# Run tests
make test

# Run linters
make lint

# Show version info
make version

# Development targets
make dev-ui      # Start UI dev server
make dev-server  # Start Go server without UI
```

### Using Build Script

```bash
# Full build
./scripts/build.sh

# UI only
./scripts/build.sh ui

# Go binary only
./scripts/build.sh go

# Clean
./scripts/build.sh clean

# Help
./scripts/build.sh help
```

## Build Process

The automated build process:

1. **UI Build**: Builds the React UI using Vite
   - Installs dependencies with `yarn install --frozen-lockfile`
   - Builds production bundle with `yarn build`
   - Outputs to `ui/dist/`

2. **UI Embedding**: Copies UI build to Go embed location
   - Copies `ui/dist/` to `internal/api/ui/`
   - Go embed directive picks up files from this location

3. **Go Build**: Builds the Go binary with embedded UI
   - Sets CGO flags for FAISS and ONNX Runtime
   - Embeds UI files using `//go:embed` directive
   - Outputs `semango` binary

## Development Workflow

### UI Development
```bash
# Start UI dev server (hot reload)
make dev-ui
# or
cd ui && yarn dev
```

### Backend Development
```bash
# Build and run without UI (faster iteration)
make dev-server
# or
make build-no-ui && ./semango server
```

### Full Development
```bash
# Build everything and run
make run
```

## Build Artifacts

- `semango` - Main binary with embedded UI
- `ui/dist/` - UI build output (gitignored)
- `internal/api/ui/` - UI files for Go embedding (gitignored)

## CI/CD Integration

The build process is designed for CI/CD:

```yaml
# Example GitHub Actions step
- name: Build Semango
  run: make build

# Or using the script
- name: Build Semango
  run: ./scripts/build.sh
```

## Troubleshooting

### UI Build Fails
- Ensure Node.js 18+ and Yarn are installed
- Check `ui/package.json` for dependency issues
- Run `cd ui && yarn install` manually

### Go Build Fails
- Ensure CGO is enabled
- Check FAISS and ONNX Runtime library paths
- Verify Go 1.21+ is installed

### Assets Not Loading
- Ensure UI build completed successfully
- Check that `internal/api/ui/` contains the built files
- Verify Go embed directive is working

## Manual Build Steps

If you need to build manually:

```bash
# 1. Build UI
cd ui
yarn install
yarn build
cd ..

# 2. Copy UI to embed location
rm -rf internal/api/ui
cp -r ui/dist internal/api/ui

# 3. Build Go binary
export CGO_LDFLAGS="-L/app/libs -lfaiss_c -lonnxruntime -Wl,-rpath,/app/libs"
go build -o semango ./cmd/semango
``` 