.PHONY: build run clean test ui-build ui-clean all lint version

BINARY_NAME=semango
CMD_PATH=./cmd/semango
UI_DIR=ui
UI_DIST_DIR=$(UI_DIR)/dist
EMBED_UI_DIR=internal/api/ui

# Version info
# Prefer the latest git tag (Go best practice; also what Prepress uses),
# then fall back to VERSION file for source archives, else "dev".
VERSION ?= $(shell \
	TAG=$$(git describe --tags --abbrev=0 2>/dev/null); \
	if [ -n "$$TAG" ]; then echo "$$TAG" | sed 's/^v//'; \
	elif [ -f VERSION ]; then cat VERSION; \
	else echo "dev"; fi)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

LIBS_DIR ?= $(CURDIR)/libs
FAISS_STATIC_DIR ?= $(CURDIR)/libs-static
ONNX_EMBED_DIR=internal/onnxruntime/embedded
ONNX_VERSION ?= 1.22.0

comma := ,

# Platform-specific CGO flags
FAISS_STATIC_C := $(wildcard $(FAISS_STATIC_DIR)/libfaiss_c.a)
FAISS_STATIC := $(wildcard $(FAISS_STATIC_DIR)/libfaiss.a)

ifeq ($(OS),Windows_NT)
	RPATH_FLAG =
	BINARY_EXT = .exe
	CGO_LDFLAGS_FAISS =
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Darwin)
		# GitHub Actions runners sometimes don't have `brew` on PATH for Make's $(shell ...).
		# Prefer standard Homebrew prefixes and only include paths that exist.
		LIBOMP_LIBDIR := $(firstword $(wildcard /opt/homebrew/opt/libomp/lib /usr/local/opt/libomp/lib))
		OPENBLAS_LIBDIR := $(firstword $(wildcard /opt/homebrew/opt/openblas/lib /usr/local/opt/openblas/lib))
		LIBOMP_LDFLAGS := $(if $(LIBOMP_LIBDIR),-L$(LIBOMP_LIBDIR),)
		OPENBLAS_LDFLAGS := $(if $(OPENBLAS_LIBDIR),-L$(OPENBLAS_LIBDIR),)
		LIBOMP_RPATH := $(if $(LIBOMP_LIBDIR),-Wl$(comma)-rpath$(comma)$(LIBOMP_LIBDIR),)
		OPENBLAS_RPATH := $(if $(OPENBLAS_LIBDIR),-Wl$(comma)-rpath$(comma)$(OPENBLAS_LIBDIR),)
		RPATH_FLAG = -Wl,-rpath,@loader_path/libs
		FAISS_STATIC_EXTRA = -lc++ \
			$(LIBOMP_LDFLAGS) -lomp \
			$(OPENBLAS_LDFLAGS) -lopenblas \
			$(LIBOMP_RPATH) \
			$(OPENBLAS_RPATH)
	else
		RPATH_FLAG = -Wl,-rpath,'$$ORIGIN/libs'
		FAISS_STATIC_EXTRA = -lstdc++ -lgomp -lopenblas -lpthread -lm
	endif
	BINARY_EXT =
	ifneq ($(FAISS_STATIC_C),)
	ifneq ($(FAISS_STATIC),)
		ifeq ($(UNAME_S),Darwin)
			# Some CGO deps also add "-lfaiss_c"; ensure it resolves to the archive in libs-static.
			CGO_LDFLAGS_FAISS = -L$(FAISS_STATIC_DIR) -lfaiss_c -lfaiss $(FAISS_STATIC_EXTRA)
		else
			CGO_LDFLAGS_FAISS = -L$(FAISS_STATIC_DIR) -Wl,-Bstatic -l:libfaiss_c.a -l:libfaiss.a -Wl,-Bdynamic $(FAISS_STATIC_EXTRA)
		endif
	else
		CGO_LDFLAGS_FAISS = -L$(LIBS_DIR) -lfaiss_c -lfaiss $(RPATH_FLAG)
	endif
	else
		CGO_LDFLAGS_FAISS = -L$(LIBS_DIR) -lfaiss_c -lfaiss $(RPATH_FLAG)
	endif
endif

CGO_LDFLAGS_ALL=$(CGO_LDFLAGS_FAISS)
CGO_CPPFLAGS_ALL=-I$(CURDIR) -I$(CURDIR)/include

all: build


onnx-embed:
	@echo "Preparing embedded ONNX Runtime library..."
	@mkdir -p $(ONNX_EMBED_DIR)
	@if [ "$(OS)" = "Windows_NT" ]; then \
		if [ -f "$(LIBS_DIR)/onnxruntime.dll" ]; then \
			cp -f "$(LIBS_DIR)/onnxruntime.dll" "$(ONNX_EMBED_DIR)/onnxruntime.dll"; \
		else \
			echo "ONNX Runtime DLL not found in $(LIBS_DIR)"; \
			exit 1; \
		fi; \
	elif [ "$(UNAME_S)" = "Darwin" ]; then \
		if [ -f "$(LIBS_DIR)/libonnxruntime.$(ONNX_VERSION).dylib" ]; then \
			cp -f "$(LIBS_DIR)/libonnxruntime.$(ONNX_VERSION).dylib" "$(ONNX_EMBED_DIR)/libonnxruntime.$(ONNX_VERSION).dylib"; \
		else \
			echo "ONNX Runtime dylib not found in $(LIBS_DIR)"; \
			exit 1; \
		fi; \
	else \
		if [ -f "$(LIBS_DIR)/libonnxruntime.so.$(ONNX_VERSION)" ]; then \
			cp -f "$(LIBS_DIR)/libonnxruntime.so.$(ONNX_VERSION)" "$(ONNX_EMBED_DIR)/libonnxruntime.so.$(ONNX_VERSION)"; \
		elif [ -f "$(LIBS_DIR)/libonnxruntime.so" ]; then \
			cp -f "$(LIBS_DIR)/libonnxruntime.so" "$(ONNX_EMBED_DIR)/libonnxruntime.so.$(ONNX_VERSION)"; \
		else \
			echo "ONNX Runtime .so not found in $(LIBS_DIR)"; \
			exit 1; \
		fi; \
	fi
	@echo "Embedded ONNX Runtime ready."

test: onnx-embed
	CGO_CPPFLAGS="$(CGO_CPPFLAGS_ALL)" CGO_LDFLAGS="$(CGO_LDFLAGS_ALL)" go test ./...

# Build the React UI
ui-build:
	@echo "Building React UI..."
	@cd $(UI_DIR) && yarn install --frozen-lockfile
	@cd $(UI_DIR) && yarn build
	@echo "React UI built successfully."

# Copy UI build to embed location
ui-copy: ui-build
	@echo "Copying UI build to embed location..."
	@rm -rf $(EMBED_UI_DIR)
	@cp -r $(UI_DIST_DIR) $(EMBED_UI_DIR)
	@echo "UI copied to $(EMBED_UI_DIR)"

# Build the Go binary with embedded UI
build: ui-copy onnx-embed
	@echo "Building $(BINARY_NAME)$(BINARY_EXT) with embedded UI..."
	@CGO_CPPFLAGS="$(CGO_CPPFLAGS_ALL)" CGO_LDFLAGS="$(CGO_LDFLAGS_ALL)" go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)$(BINARY_EXT) $(CMD_PATH)
	@echo "$(BINARY_NAME)$(BINARY_EXT) built successfully with embedded UI."

# Build Go binary without UI (for development)
build-no-ui: onnx-embed
	@echo "Building $(BINARY_NAME)$(BINARY_EXT) without UI..."
	@CGO_CPPFLAGS="$(CGO_CPPFLAGS_ALL)" CGO_LDFLAGS="$(CGO_LDFLAGS_ALL)" go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)$(BINARY_EXT) $(CMD_PATH)
	@echo "$(BINARY_NAME)$(BINARY_EXT) built successfully."

# Run linters
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...
	@echo "Running UI lint..."
	@cd $(UI_DIR) && yarn lint

# Print version info
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

# Clean UI build artifacts
ui-clean:
	@echo "Cleaning UI build artifacts..."
	@rm -rf $(UI_DIST_DIR)
	@rm -rf $(EMBED_UI_DIR)
	@cd $(UI_DIR) && rm -rf node_modules

clean: ui-clean
	@echo "Cleaning up..."
	@go clean
	@rm -f $(BINARY_NAME)
	@rm -rf $(ONNX_EMBED_DIR)
	@echo "Cleanup complete."

# Development targets
dev-ui:
	@echo "Starting UI development server..."
	@cd $(UI_DIR) && yarn dev

dev-server: build-no-ui
	@echo "Starting development server..."
	@./$(BINARY_NAME) server 

