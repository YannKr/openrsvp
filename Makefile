BINARY_NAME := openrsvp
BUILD_DIR := ./bin
CMD_DIR := ./cmd/openrsvp
CGO_ENABLED := 1

.PHONY: all build dev test clean lint lint-routes frontend embed

all: lint test build

frontend:
	@echo "Building frontend..."
	cd web && npm run build

embed: frontend
	@echo "Embedding frontend..."
	rm -rf internal/server/frontend
	mkdir -p internal/server/frontend
	cp -r web/build/* internal/server/frontend/

build: embed
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

dev:
	@echo "Running in development mode..."
	CGO_ENABLED=$(CGO_ENABLED) go run $(CMD_DIR)

test:
	@echo "Running tests..."
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -v -race

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf internal/server/frontend
	rm -f openrsvp.db

lint:
	@echo "Running linter..."
	golangci-lint run ./...

lint-routes:
	@./scripts/lint-api-routes.sh
