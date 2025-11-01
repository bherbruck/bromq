.PHONY: help install run stop dev-up dev-down prod-up prod-down logs clean test test-web test-all backend frontend

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
	go build -o $@ .

# Convenience targets
build: bin/bromq ## Build the complete application

run: bin/bromq stop ## Run the server locally
	./bin/bromq

stop: ## Stop any running bromq processes
	@echo "Stopping bromq processes..."
	@-killall bromq 2>/dev/null || echo "No bromq processes found"

dev-up: ## Start development environment (hot reload)
	docker compose -f compose.dev.yml up -d
	@echo ""
	@echo "Development servers starting..."
	@echo "  Backend:  http://localhost:8080"
	@echo "  Frontend: http://localhost:5173"
	@echo ""
	@echo "View logs: make logs"

dev-down: ## Stop development environment
	docker compose -f compose.dev.yml down

prod-up: ## Start production environment
	docker compose up -d --build
	@echo ""
	@echo "Production server starting..."
	@echo "  MQTT TCP:       localhost:1883"
	@echo "  MQTT WebSocket: localhost:8883"
	@echo "  Web UI:         http://localhost:8080"
	@echo ""
	@echo "Default credentials: admin / admin"
	@echo "View logs: make logs"

prod-down: ## Stop production environment
	docker compose down

logs: ## Tail logs (add SERVICE=name to filter)
	@if [ -n "$(SERVICE)" ]; then \
		docker compose logs -f $(SERVICE); \
	else \
		docker compose logs -f; \
	fi

clean: ## Clean build artifacts and volumes
	rm -rf bin/ web/dist/ web/node_modules/
	docker compose down -v
	docker compose -f compose.dev.yml down -v

test: ## Run Go tests
	go test -v ./...

test-web: web/node_modules ## Run frontend tests
	cd web && npm test

test-all: web/node_modules ## Run all tests (Go + frontend)
	@echo "Running Go tests..."
	go test -v ./...
	@echo ""
	@echo "Running frontend tests..."
	cd web && npm test

frontend: web/dist ## Build frontend only

backend: bin/bromq ## Build backend only
