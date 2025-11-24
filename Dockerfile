# Multi-stage build for BroMQ with embedded frontend

# Stage 1: Build frontend (always on amd64 since output is platform-agnostic)
FROM --platform=linux/amd64 node:22-alpine AS frontend
WORKDIR /app/web

# Copy package files
COPY web/package*.json ./

# Install dependencies
RUN npm ci --prefer-offline --no-audit

# Copy frontend source
COPY web/ ./

# Build frontend (outputs to dist/client/)
RUN npm run build

# Stage 2: Build Go application
FROM golang:1.25-alpine AS backend
WORKDIR /app

# Accept version as build argument
ARG VERSION=dev

# Copy Go modules files
COPY go.mod go.sum ./
RUN go mod download

# Copy only Go source files
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY hooks/ ./hooks/

# Copy web package embed file
COPY web/embed.go ./web/

# Copy built frontend from previous stage
COPY --from=frontend /app/web/dist/client ./web/dist/client

# Install swag CLI and generate OpenAPI documentation
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init -g internal/api/doc.go -d ./ --output internal/api/swagger --parseDependency --parseInternal --outputTypes json,yaml

# Build the application with optimizations (pure Go, no CGO!)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o bromq ./cmd/server

# Stage 3: Runtime image
FROM alpine:latest
WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Copy binary from builder
COPY --from=backend /app/bromq .

# Create data directory
RUN mkdir -p /app/data

# Default database configuration (SQLite in mounted volume)
ENV DB_TYPE=sqlite \
    DB_PATH=/app/data/bromq.db \
    BADGER_PATH=/app/data/badger

# Expose ports
# 1883: MQTT TCP
# 8883: MQTT WebSocket
# 8080: HTTP API & Web UI
EXPOSE 1883 8883 8080

# Run as non-root user
RUN addgroup -g 1000 mqtt && \
    adduser -D -u 1000 -G mqtt mqtt && \
    chown -R mqtt:mqtt /app

USER mqtt

# Health check - verify HTTP server is responding
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Start server
# Database configured via environment variables (see compose.yml)
CMD ["/app/bromq"]
