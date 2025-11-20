.PHONY: help install run stop clean test test-race test-coverage test-web test-all lint security-deps security-code security ci schema swagger swagger-install

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
bin/bromq: $(shell find . -name '*.go' -not -path './web/*') go.mod go.sum web/dist internal/api/swagger/swagger.json
	@echo "Building Go binary..."
	@mkdir -p bin
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	go build -ldflags="-X main.version=$$VERSION" -o $@ ./cmd/server

# Generate swagger JSON/YAML specs (skip bloated docs.go)
internal/api/swagger/swagger.json: $(shell find internal/api -name '*.go' -not -path '*/swagger/*')
	@echo "Generating Swagger documentation..."
	@command -v swag >/dev/null 2>&1 || { \
		echo "Installing swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	}
	@mkdir -p internal/api/swagger
	@swag init -g internal/api/doc.go -d ./ --output internal/api/swagger --parseDependency --parseInternal --outputTypes json,yaml

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

test-race: web/dist ## Run Go tests with race detection (like CI)
	go test -race -v ./...

test-coverage: web/dist ## Run Go tests with coverage report
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "View detailed coverage in browser:"
	@echo "  go tool cover -html=coverage.out"

test-web: web/node_modules ## Run frontend tests
	cd web && npm test

test-all: web/node_modules ## Run all tests (Go + frontend)
	@echo "Running Go tests..."
	go test ./...
	@echo ""
	@echo "Running frontend tests..."
	cd web && npm test

lint: web/dist ## Run golangci-lint (like CI)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	}
	golangci-lint run --timeout=5m

security-deps: web/dist ## Run govulncheck (dependency vulnerability scan)
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

security-code: web/dist ## Run gosec (code security scan)
	@command -v gosec >/dev/null 2>&1 || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude=G404 ./...

security: web/dist ## Run all security scans (govulncheck + gosec, like CI)
	@echo "==> Running dependency vulnerability scan (govulncheck)..."
	$(MAKE) security-deps
	@echo ""
	@echo "==> Running code security scan (gosec)..."
	$(MAKE) security-code

ci: web/dist ## Run all CI checks locally (tests, lint, security)
	@echo "==> Running Go vet..."
	go vet ./...
	@echo ""
	@echo "==> Running tests with race detection..."
	go test -race -v ./...
	@echo ""
	@echo "==> Running frontend tests..."
	cd web && npm test
	@echo ""
	@echo "==> Running linter..."
	$(MAKE) lint
	@echo ""
	@echo "==> Running security scans..."
	$(MAKE) security
	@echo ""
	@echo "✅ All CI checks passed!"

schema: ## Generate JSON Schema for config files
	@echo "Generating JSON Schema..."
	@mkdir -p schema
	@go run cmd/schema-gen/main.go > schema/bromq-config.schema.json
	@echo "Schema generated: schema/bromq-config.schema.json"

swagger-install: ## Install swag CLI tool for generating OpenAPI docs
	@echo "Installing swag..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "✅ swag installed"

swagger: ## Generate OpenAPI/Swagger documentation
	@command -v swag >/dev/null 2>&1 || { \
		echo "swag not installed, installing..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	}
	@echo "Generating Swagger documentation..."
	@mkdir -p internal/api/swagger
	@swag init -g internal/api/doc.go -d ./ --output internal/api/swagger --parseDependency --parseInternal --outputTypes json,yaml
	@echo "✅ Swagger JSON/YAML generated in internal/api/swagger/"
	@echo "   Access at: http://localhost:8080/swagger/index.html"
