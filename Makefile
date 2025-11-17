.PHONY: help install run stop clean test test-web test-all schema

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Smart dependency tracking: only install npm packages when needed
web/node_modules: web/package.json web/package-lock.json
	@echo "Installing npm dependencies..."
	cd web && npm ci
	@touch $@

install: ## Install/update npm dependencies (for adding new packages)
	cd web && npm install

# Frontend build output (tracks all files in web/app directory)
web/dist: web/node_modules $(shell find web/app -type f 2>/dev/null || echo "web/app")
	@echo "Building frontend..."
	cd web && npm run build
	@touch $@

# Go binary (tracks all Go source files and embeds frontend)
bin/bromq: $(shell find . -name '*.go' -not -path './web/*') go.mod go.sum web/dist
	@echo "Building Go binary..."
	@mkdir -p bin
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	go build -ldflags="-X main.version=$$VERSION" -o $@ ./cmd/server

# Convenience targets
build: bin/bromq ## Build the complete application

run: bin/bromq stop ## Run the server locally
	./bin/bromq

stop: ## Stop any running bromq processes
	@echo "Stopping bromq processes..."
	@-killall bromq 2>/dev/null || echo "No bromq processes found"

clean: ## Clean build artifacts and volumes
	rm -rf bin/ web/dist/ web/node_modules/
	docker compose down -v
	docker compose -f compose.dev.yml down -v

test: ## Run Go tests
	go test ./...

test-web: web/node_modules ## Run frontend tests
	cd web && npm test

test-all: web/node_modules ## Run all tests (Go + frontend)
	@echo "Running Go tests..."
	go test ./...
	@echo ""
	@echo "Running frontend tests..."
	cd web && npm test

schema: ## Generate JSON Schema for config files
	@echo "Generating JSON Schema..."
	@mkdir -p schema
	@go run cmd/schema-gen/main.go > schema/bromq-config.schema.json
	@echo "Schema generated: schema/bromq-config.schema.json"
