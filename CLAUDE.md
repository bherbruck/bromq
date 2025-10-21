# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A single-node MQTT broker with embedded web UI built on mochi-mqtt/server. Features include database-backed authentication, granular ACL permissions, REST API, and a web dashboard (Vite + shadcn/ui). The entire application compiles to a single binary with the frontend embedded.

## Tech Stack

**Backend:**
- mochi-mqtt/server v2 - MQTT broker core
- SQLite (modernc.org/sqlite) - Embedded database for users/ACL
- stdlib net/http (Go 1.22+) - HTTP server and routing
- JWT (golang-jwt/jwt/v5) - API authentication

**Frontend:** (to be scaffolded in `web/`)
- Vite - Build tool
- shadcn/ui - Component library
- Frontend embeds into Go binary via `go:embed`

## Project Structure

```
mqtt-server/
├── main.go                      # Entry point, wires everything together
├── internal/
│   ├── storage/                # SQLite database layer
│   │   ├── db.go              # Connection, schema, migrations
│   │   ├── users.go           # User CRUD + authentication
│   │   └── acl.go             # ACL rules CRUD + topic matching
│   ├── mqtt/                   # MQTT server wrapper
│   │   ├── config.go          # Configuration struct
│   │   ├── server.go          # Server initialization
│   │   └── metrics.go         # Stats/metrics extraction
│   └── api/                    # REST API
│       ├── models.go          # Request/response types
│       ├── middleware.go      # JWT auth, CORS, logging, admin guard
│       ├── handlers.go        # HTTP handlers for all endpoints
│       └── server.go          # HTTP server setup + routing
├── hooks/auth/
│   ├── auth.go                # MQTT authentication hook (OnConnectAuthenticate)
│   └── acl.go                 # MQTT ACL hook (OnACLCheck)
├── web/                        # Frontend (scaffolded by you)
│   └── dist/                  # Build output (embedded via go:embed)
├── Dockerfile                  # Multi-stage build (Node → Go → Alpine)
└── go.mod
```

## Common Commands

### Using Makefile (Recommended)

```bash
make help          # Show all available commands

# Development
make dev-up        # Start development environment (hot reload)
make dev-down      # Stop development environment
make logs          # View logs

# Production
make prod-up       # Start production environment
make prod-down     # Stop production environment

# Local development (without Docker)
make build         # Build frontend + Go binary
make run           # Build and run locally

# Utilities
make clean         # Clean build artifacts and volumes
make test          # Run Go tests
```

### Docker Compose

```bash
# Production (recommended for deployment)
docker compose up -d              # Start in background
docker compose logs -f            # Follow logs
docker compose down               # Stop and remove containers
docker compose down -v            # Stop and remove volumes

# Development (hot reload)
docker compose -f compose.dev.yml up -d
docker compose -f compose.dev.yml down
```

### Manual Commands

```bash
# Build (without frontend)
go build -o bin/mqtt-server .

# Run server
./bin/mqtt-server
# Or with custom flags:
./bin/mqtt-server -db /path/to/db.sqlite -http :8080 -mqtt-tcp :1883 -mqtt-ws :8883

# Run directly
go run .

# Build with embedded frontend
cd web && npm run build && cd ..
go build -o bin/mqtt-server .

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
```

## Architecture & Key Concepts

### 1. Database Layer (`internal/storage`)

**Schema:**
- `users`: id, username, password_hash (bcrypt), role (user/admin), created_at
- `acl_rules`: id, user_id, topic_pattern, permission (pub/sub/pubsub)

**Key functions:**
- `storage.Open(path)` - Opens SQLite, creates schema, adds default admin user
- `db.AuthenticateUser(username, password)` - Validates credentials
- `db.CheckACL(username, topic, action)` - Checks if user can pub/sub to topic
- Topic matching supports MQTT wildcards: `+` (single level), `#` (multi-level)

**Default credentials:** `admin` / `admin` (auto-created on first run)

### 2. MQTT Hooks (`hooks/auth`)

Hooks implement the mochi-mqtt hook interface to intercept MQTT lifecycle events:

**AuthHook** (`auth.go`):
- Implements `OnConnectAuthenticate` to validate MQTT client credentials against database
- Anonymous connections allowed if no username provided
- Stores username in `cl.Properties.Username` for ACL checks

**ACLHook** (`acl.go`):
- Implements `OnACLCheck` to authorize publish/subscribe operations
- Reads username from `cl.Properties.Username`
- Calls `db.CheckACL()` to validate against stored rules
- Admin users have full access to all topics

### 3. MQTT Server (`internal/mqtt`)

**Config options:**
- TCPAddr: MQTT TCP listener (default `:1883`)
- WSAddr: WebSocket listener (default `:8883`)
- EnableTLS: TLS support (not yet implemented)
- MaxClients: Connection limit (0 = unlimited)
- RetainAvailable: Enable retained messages

**Key functions:**
- `mqtt.New(config)` - Creates server instance
- `server.AddAuthHook()` - Registers authentication hook
- `server.AddACLHook()` - Registers ACL hook
- `server.Start()` - Starts all listeners
- `server.GetClients()` - Returns connected clients info
- `server.GetMetrics()` - Returns server statistics

### 4. REST API (`internal/api`)

**Authentication:** JWT tokens (24h expiry)
- Header: `Authorization: Bearer <token>`
- Middleware: `AuthMiddleware` validates JWT, adds claims to context
- Admin guard: `AdminOnly` middleware restricts endpoints

**Endpoints:**

Public:
- `POST /api/auth/login` - Get JWT token

Protected (any authenticated user):
- `GET /api/users` - List all users
- `GET /api/acl` - List ACL rules
- `GET /api/clients` - List connected MQTT clients
- `GET /api/metrics` - Get server metrics

Protected (admin only):
- `POST /api/users` - Create user
- `PUT /api/users/{id}` - Update user
- `DELETE /api/users/{id}` - Delete user
- `POST /api/acl` - Create ACL rule
- `DELETE /api/acl/{id}` - Delete ACL rule
- `POST /api/clients/{id}/disconnect` - Force disconnect client

**Frontend serving:**
- Root path (`/`) serves embedded SPA from `web/dist`
- Falls back to `index.html` for client-side routing

### 5. Main Entry Point (`cmd/server/main.go`)

Orchestrates startup:
1. Parse CLI flags
2. Open SQLite database
3. Create MQTT server with config
4. Register auth + ACL hooks
5. Start MQTT server (goroutine)
6. Start HTTP API server (goroutine)
7. Wait for SIGINT/SIGTERM

### 6. Frontend Integration

**Setup (React Router v7 + shadcn/ui):**
```bash
cd web
npm install
npm run build  # Outputs to web/dist/client/

# The react-router.config.ts is configured for SPA mode:
# - ssr: false (static builds, no server)
# - buildDirectory: './dist' (outputs to dist/client/)
```

**Embedding:**
- `main.go` has `//go:embed all:web/dist/client`
- Binary includes all frontend assets
- `api.Server` serves files via `http.FileServer`
- React Router v7 in SPA mode with client-side routing

## Development Workflow

### Option 1: Docker Compose Development (Recommended)

**Easiest way to get started with hot reload for both frontend and backend:**

```bash
# Start everything
make dev-up
# or: docker compose -f compose.dev.yml up -d

# Access points:
# - Backend API: http://localhost:8080
# - Frontend:    http://localhost:5173 (with HMR)

# View logs
make logs
# or: docker compose logs -f

# Stop everything
make dev-down
```

**Features:**
- ✅ Automatic Go code reload (via volume mount)
- ✅ Vite HMR for instant React updates
- ✅ Persistent database in Docker volume
- ✅ No need to install Go/Node locally

### Option 2: Local Development (No Docker)

**Terminal 1 - Start Go backend:**
```bash
go run .
# API server running on http://localhost:8080
```

**Terminal 2 - Start Vite dev server:**
```bash
cd web
npm run dev
# Frontend with HMR on http://localhost:5173
# API requests to /api/* automatically proxied to :8080
```

**Frontend Dev Server Proxy:**
The `vite.config.ts` includes proxy configuration:
```typescript
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8080',  // Go backend
      changeOrigin: true,
    }
  }
}
```

This means:
- Visit http://localhost:5173 for frontend development
- All `/api/*` requests are automatically forwarded to the Go backend
- No CORS issues during development
- Hot module reloading for instant React updates

### Production Deployment

**Option 1: Docker Compose (Recommended)**
```bash
make prod-up
# or: docker compose up -d --build

# Everything available at:
# - MQTT TCP:       localhost:1883
# - MQTT WebSocket: localhost:8883
# - Web UI + API:   http://localhost:8080
```

**Option 2: Manual Build**
```bash
# Build
cd web && npm run build && cd ..
go build -o bin/mqtt-server .

# Deploy single binary (17MB)
scp bin/mqtt-server user@server:/opt/mqtt-server/
ssh user@server "/opt/mqtt-server/mqtt-server"
```

**Option 3: Docker Only**
```bash
docker build -t mqtt-server .
docker run -d \
  -p 1883:1883 \
  -p 8883:8883 \
  -p 8080:8080 \
  -v mqtt-data:/app/data \
  --name mqtt-server \
  mqtt-server
```

## Important Implementation Details

**JWT Secret:** Currently hardcoded in `internal/api/middleware.go` - move to environment variable for production

**Anonymous MQTT access:** Enabled by default in auth hook - ACL still enforced

**Admin users:** Bypass ACL checks entirely (see `storage/acl.go:72`)

**Topic wildcards:**
- `sensor/+/temperature` matches `sensor/living-room/temperature`
- `device/#` matches `device/1/status` and `device/2/info/version`
- See `matchTopic()` in `storage/acl.go` for implementation

**Error handling:** All handlers return JSON errors with appropriate HTTP status codes

**CORS:** Enabled for all origins in `CORSMiddleware` - restrict for production

## Testing MQTT

```bash
# Install mosquitto clients
sudo apt-get install mosquitto-clients  # or brew install mosquitto

# Test anonymous connection
mosquitto_pub -h localhost -p 1883 -t "test/topic" -m "hello"

# Test with authentication
mosquitto_pub -h localhost -p 1883 -u admin -P admin -t "test/topic" -m "hello"

# Subscribe
mosquitto_sub -h localhost -p 1883 -u admin -P admin -t "test/#"
```

## Testing API

```bash
# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Save token from response, then:
TOKEN="<your-token>"

# List users
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/auth/users

# Create ACL rule
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id":1,"topic_pattern":"sensor/+/temp","permission":"pubsub"}'

# Get metrics
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/metrics
```

## Security Considerations

- Change default admin password immediately after first run
- Use TLS for production deployments (not yet implemented)
- Store JWT secret in environment variable, not code
- Implement rate limiting on API endpoints for production
- Consider using proper secrets management for production databases
- Review CORS policy before production deployment
