# Multi-stage build for MQTT Server with embedded frontend

# Stage 1: Build frontend
FROM node:22-alpine AS frontend
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

# Copy Go modules files
COPY go.mod go.sum ./
RUN go mod download

# Copy only Go source files
COPY main.go ./
COPY internal/ ./internal/
COPY hooks/ ./hooks/

# Copy web package embed file
COPY web/embed.go ./web/

# Copy built frontend from previous stage
COPY --from=frontend /app/web/dist/client ./web/dist/client

# Build the application with optimizations (pure Go, no CGO!)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o mqtt-server .

# Stage 3: Runtime image
FROM alpine:latest
WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Copy binary from builder
COPY --from=backend /app/mqtt-server .

# Create data directory
RUN mkdir -p /app/data

# Default database configuration (SQLite in mounted volume)
ENV DB_TYPE=sqlite \
    DB_PATH=/app/data/mqtt-server.db

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
CMD ["/app/mqtt-server"]
