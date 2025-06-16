.PHONY: build run clean

BINARY_NAME=semango
CMD_PATH=./cmd/semango

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) $(CMD_PATH)/main.go
	@echo "$(BINARY_NAME) built successfully."

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

clean:
	@echo "Cleaning up..."
	@go clean
	@rm -f $(BINARY_NAME)
	@echo "Cleanup complete." 