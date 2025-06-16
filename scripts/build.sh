#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
UI_DIR="ui"
UI_DIST_DIR="$UI_DIR/dist"
EMBED_UI_DIR="internal/api/ui"
BINARY_NAME="semango"
CMD_PATH="./cmd/semango"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if yarn is installed
check_yarn() {
    if ! command -v yarn &> /dev/null; then
        log_error "yarn is not installed. Please install yarn first."
        exit 1
    fi
}

# Check if go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go first."
        exit 1
    fi
}

# Build UI
build_ui() {
    log_info "Building React UI..."
    
    if [ ! -d "$UI_DIR" ]; then
        log_error "UI directory '$UI_DIR' not found"
        exit 1
    fi
    
    cd "$UI_DIR"
    
    # Install dependencies
    log_info "Installing UI dependencies..."
    yarn install --frozen-lockfile
    
    # Build UI
    log_info "Building UI..."
    yarn build
    
    cd ..
    
    if [ ! -d "$UI_DIST_DIR" ]; then
        log_error "UI build failed - dist directory not found"
        exit 1
    fi
    
    log_info "UI build completed successfully"
}

# Copy UI to embed location
copy_ui() {
    log_info "Copying UI build to embed location..."
    
    # Remove existing embedded UI
    if [ -d "$EMBED_UI_DIR" ]; then
        rm -rf "$EMBED_UI_DIR"
    fi
    
    # Copy new UI build
    cp -r "$UI_DIST_DIR" "$EMBED_UI_DIR"
    
    log_info "UI copied to $EMBED_UI_DIR"
}

# Build Go binary
build_go() {
    log_info "Building Go binary with embedded UI..."
    
    # Set CGO flags for FAISS and ONNX
    export CGO_LDFLAGS="-L/app/libs -lfaiss_c -lonnxruntime -Wl,-rpath,/app/libs"
    
    go build -o "$BINARY_NAME" "$CMD_PATH"
    
    if [ ! -f "$BINARY_NAME" ]; then
        log_error "Go build failed - binary not found"
        exit 1
    fi
    
    log_info "Go binary built successfully: $BINARY_NAME"
}

# Clean build artifacts
clean() {
    log_info "Cleaning build artifacts..."
    
    # Clean Go artifacts
    go clean
    rm -f "$BINARY_NAME"
    
    # Clean UI artifacts
    rm -rf "$UI_DIST_DIR"
    rm -rf "$EMBED_UI_DIR"
    
    log_info "Cleanup completed"
}

# Main build process
main() {
    case "${1:-build}" in
        "build")
            log_info "Starting full build process..."
            check_yarn
            check_go
            build_ui
            copy_ui
            build_go
            log_info "Build completed successfully! Binary: $BINARY_NAME"
            ;;
        "ui")
            log_info "Building UI only..."
            check_yarn
            build_ui
            copy_ui
            log_info "UI build completed"
            ;;
        "go")
            log_info "Building Go binary only..."
            check_go
            build_go
            log_info "Go build completed"
            ;;
        "clean")
            clean
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [build|ui|go|clean|help]"
            echo ""
            echo "Commands:"
            echo "  build (default) - Build UI and Go binary"
            echo "  ui              - Build UI only"
            echo "  go              - Build Go binary only"
            echo "  clean           - Clean build artifacts"
            echo "  help            - Show this help"
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

main "$@" 