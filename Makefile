.PHONY: build run clean test ui-build ui-clean all

BINARY_NAME=semango
CMD_PATH=./cmd/semango
UI_DIR=ui
UI_DIST_DIR=$(UI_DIR)/dist
EMBED_UI_DIR=internal/api/ui

CGO_LDFLAGS_FAISS=-L/app/libs -lfaiss_c -Wl,-rpath,/app/libs
CGO_LDFLAGS_ONNX=-L/app/libs -lonnxruntime -Wl,-rpath,/app/libs
CGO_LDFLAGS_ALL=$(CGO_LDFLAGS_FAISS) $(CGO_LDFLAGS_ONNX)

all: build

test:
	CGO_LDFLAGS="$(CGO_LDFLAGS_ALL)" go test ./...

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
build: ui-copy
	@echo "Building $(BINARY_NAME) with embedded UI..."
	@CGO_LDFLAGS="$(CGO_LDFLAGS_ALL)" go build -o $(BINARY_NAME) $(CMD_PATH)
	@echo "$(BINARY_NAME) built successfully with embedded UI."

# Build Go binary without UI (for development)
build-no-ui:
	@echo "Building $(BINARY_NAME) without UI..."
	@CGO_LDFLAGS="$(CGO_LDFLAGS_ALL)" go build -o $(BINARY_NAME) $(CMD_PATH)
	@echo "$(BINARY_NAME) built successfully."

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
	@echo "Cleanup complete."

# Development targets
dev-ui:
	@echo "Starting UI development server..."
	@cd $(UI_DIR) && yarn dev

dev-server: build-no-ui
	@echo "Starting development server..."
	@./$(BINARY_NAME) server 

