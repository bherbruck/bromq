.PHONY: help build run stop dev-up dev-down prod-up prod-down logs clean test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary locally
	cd web && npm run build
	go build -o bin/bromq .

run: build stop ## Run the server locally
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

frontend: ## Build frontend only
	cd web && npm run build

backend: ## Build backend only
	go build -o bin/bromq .
