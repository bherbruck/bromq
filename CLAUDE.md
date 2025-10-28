# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A single-node MQTT broker with embedded web UI built on mochi-mqtt/server. Features include database-backed authentication, granular ACL permissions, REST API, and a web dashboard (Vite + shadcn/ui). The entire application compiles to a single binary with the frontend embedded.

## Tech Stack

**Backend:**
- mochi-mqtt/server v2 - MQTT broker core
- GORM v1.31+ - ORM with auto-migration support
- Database support: SQLite (default), PostgreSQL, MySQL
- stdlib net/http (Go 1.22+) - HTTP server and routing
- JWT (golang-jwt/jwt/v5) - API authentication
- slog (stdlib log/slog) - Structured logging with configurable levels and formats

**Frontend:**
- React Router v7 (SPA mode) - Routing and build tool
- Vite - Build tool and dev server
- shadcn/ui - Component library (Radix UI + Tailwind CSS)
- Frontend embeds into Go binary via `go:embed`

## Project Structure

```
mqtt-server/
├── main.go                      # Entry point, wires everything together
├── internal/
│   ├── storage/                    # Database layer (SQLite/PostgreSQL/MySQL)
│   │   ├── db.go                  # Connection, schema, GORM auto-migration
│   │   ├── config.go              # Database configuration (env vars + flags)
│   │   ├── models.go              # GORM models with tags
│   │   ├── dashboard_users.go     # Dashboard admin CRUD + authentication
│   │   ├── mqtt_users.go          # MQTT credentials CRUD + authentication
│   │   ├── mqtt_clients.go        # Client connection tracking CRUD
│   │   ├── acl.go                 # ACL rules CRUD + topic matching
│   │   └── retained.go            # Retained message persistence
│   ├── config/                     # Configuration file support
│   │   └── config.go              # YAML parsing with env var interpolation
│   ├── provisioning/               # Configuration provisioning
│   │   └── provisioning.go        # Sync config to database (Grafana-style)
│   ├── mqtt/                       # MQTT server wrapper
│   │   ├── config.go              # Configuration struct
│   │   ├── server.go              # Server initialization
│   │   └── metrics.go             # Stats/metrics extraction
│   └── api/                        # REST API
│       ├── models.go              # Request/response types
│       ├── middleware.go          # JWT auth, CORS, logging, admin guard
│       ├── dashboard_handlers.go  # Dashboard user management endpoints
│       ├── mqtt_handlers.go       # MQTT users/clients/ACL endpoints
│       ├── handlers.go            # Legacy endpoints + metrics
│       └── server.go              # HTTP server setup + routing
├── hooks/
│   ├── auth/
│   │   ├── auth.go                # MQTT authentication hook
│   │   └── acl.go                 # MQTT ACL authorization hook
│   ├── tracking/
│   │   └── tracking.go            # Client connection tracking hook
│   ├── metrics/
│   │   └── metrics.go             # Prometheus metrics hook
│   └── retained/
│       └── retained.go            # Retained message persistence hook
├── web/                            # Frontend (React Router v7 + shadcn/ui)
│   ├── app/                       # React application source
│   │   ├── components/           # Reusable UI components
│   │   ├── routes/               # Page routes
│   │   └── lib/                  # API client and utilities
│   └── dist/client/              # Build output (embedded via go:embed)
├── examples/
│   ├── config/
│   │   ├── config.example.yml          # Full provisioning example
│   │   ├── config.minimal.example.yml  # Minimal example
│   │   └── config.multitenant.example.yml  # Multi-tenant example
│   ├── compose.postgres.yml    # PostgreSQL compose example
│   └── compose.mysql.yml       # MySQL compose example
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
# Production with SQLite (recommended for deployment)
docker compose up -d              # Start in background
docker compose logs -f            # Follow logs
docker compose down               # Stop and remove containers
docker compose down -v            # Stop and remove volumes

# Production with PostgreSQL
docker compose -f examples/compose.postgres.yml up -d
docker compose -f examples/compose.postgres.yml down

# Production with MySQL
docker compose -f examples/compose.mysql.yml up -d
docker compose -f examples/compose.mysql.yml down

# Development (hot reload)
docker compose -f compose.dev.yml up -d
docker compose -f compose.dev.yml down
```

### Manual Commands

```bash
# Build (without frontend)
go build -o bin/mqtt-server .

# Run server (defaults to SQLite)
./bin/mqtt-server

# Run with custom database configuration
./bin/mqtt-server \
  -db-type postgres \
  -db-host localhost \
  -db-port 5432 \
  -db-user mqtt \
  -db-password secret \
  -mqtt-tcp :1883 \
  -mqtt-ws :8883 \
  -http :8080

# Run directly
go run .

# Build with embedded frontend
cd web && npm run build && cd ..
go build -o bin/mqtt-server .

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests in a specific package
go test ./internal/storage
go test ./internal/api
go test ./hooks/auth

# Run a specific test
go test -v -run TestCreateUser ./internal/storage

# Run tests without cache
go test -count=1 ./...

# Tidy dependencies
go mod tidy

# Run with configuration file
./bin/mqtt-server -config config.yml

# Set environment variables for config passwords
export SENSOR_PASSWORD="secret123"
export ADMIN_DEVICE_PASSWORD="admin456"
./bin/mqtt-server -config config.yml
```

## Configuration File (Provisioning)

The server supports YAML configuration files for provisioning MQTT users and ACL rules. The config file is the source of truth and syncs to the database on every startup (like Grafana provisioning).

**Features:**
- Environment variable interpolation: `${VAR_NAME}` or `$VAR_NAME`
- Automatic sync on startup: Create, update, and remove provisioned items
- Coexists with manually created items via API (provisioned items tracked separately)
- Provisioned items can be updated via API, but changes are overwritten on next restart

**Configuration:**
```bash
# Via command-line flag
./mqtt-server -config config.yml

# Via environment variable
CONFIG_FILE=/etc/mqtt-server/config.yml ./mqtt-server
```

**Config file structure:**
```yaml
# MQTT users (device credentials)
users:
  - username: sensor_user
    password: ${SENSOR_PASSWORD}  # Environment variable
    description: "Temperature sensors"
    metadata:
      location: "warehouse"
      device_type: "sensor"

# ACL rules (topic access control)
acl_rules:
  - mqtt_username: sensor_user
    topic_pattern: "sensors/${username}/#"  # Dynamic placeholder
    permission: pubsub  # pub, sub, or pubsub
```

**Example files:**
- `examples/config/config.example.yml` - Full example with multiple users and rules
- `examples/config/config.minimal.example.yml` - Minimal example for testing
- `examples/config/config.multitenant.example.yml` - Multi-tenant SaaS example

**Provisioning behavior:**
- **Users**: Created if missing, updated if exists (password, description, metadata)
- **ACL rules**: Old provisioned rules deleted, new rules created from config
- **Orphaned users**: Users provisioned but no longer in config are removed
- **Manual items**: Items created via API (not provisioned) are never touched
- **Modification protection**: Provisioned items **cannot be modified or deleted** via API/UI
  - API returns `409 Conflict` with helpful error message
  - Frontend UI shows "Config File" badge and disables edit/delete buttons
  - To modify: Edit the config file and restart the server

**Usage example:**
```bash
# Set passwords via environment
export SENSOR_PASSWORD="super_secret_123"
export CAMERA_PASSWORD="camera_pass_456"

# Run with config
./mqtt-server -config config.yml

# Docker Compose
services:
  mqtt-server:
    image: mqtt-server
    environment:
      - SENSOR_PASSWORD=super_secret_123
      - CAMERA_PASSWORD=camera_pass_456
    volumes:
      - ./config.yml:/app/config.yml
    command: ["-config", "/app/config.yml"]
```

## Architecture & Key Concepts

### 1. Database Layer (`internal/storage`)

**Uses GORM for ORM and Auto-Migration:**
- Models defined in `internal/storage/models.go` with GORM tags
- Auto-migration runs on startup - add columns by updating struct definitions
- **Supports multiple databases:** SQLite (default), PostgreSQL, MySQL

**Schema (Three-Table Architecture):**

The system uses a three-table architecture to separate concerns between dashboard administration, MQTT credentials, and individual device tracking:

1. **`dashboard_users`**: Dashboard administrators (human users)
   - `id` (uint, primary key)
   - `username` (string, unique, not null)
   - `password_hash` (string, not null, bcrypt)
   - `role` (string, not null, default='admin') - 'admin' or 'viewer'
   - `metadata` (jsonb) - Custom attributes
   - `created_at`, `updated_at` (timestamps)

2. **`mqtt_users`**: MQTT authentication credentials (shared by multiple devices)
   - `id` (uint, primary key)
   - `username` (string, unique, not null)
   - `password_hash` (string, not null, bcrypt)
   - `description` (text) - Human-readable description
   - `metadata` (jsonb) - Custom attributes
   - `provisioned_from_config` (boolean, default=false) - Managed by config file
   - `created_at`, `updated_at` (timestamps)

3. **`mqtt_clients`**: Individual MQTT device/client connection tracking
   - `id` (uint, primary key)
   - `client_id` (string, unique, not null) - MQTT Client ID
   - `mqtt_user_id` (uint, foreign key to mqtt_users, cascade delete)
   - `metadata` (jsonb) - Custom attributes per device
   - `first_seen`, `last_seen` (timestamps)
   - `is_active` (boolean) - Currently connected
   - `created_at`, `updated_at` (timestamps)

4. **`acl_rules`**: Access control rules for MQTT topics
   - `id` (uint, primary key)
   - `mqtt_user_id` (uint, foreign key to mqtt_users, cascade delete)
   - `topic_pattern` (string, not null) - Supports MQTT wildcards (+, #) and dynamic placeholders (${username}, ${clientid})
   - `permission` (string, not null) - 'pub', 'sub', or 'pubsub'
   - `provisioned_from_config` (boolean, default=false) - Managed by config file
   - `created_at` (timestamp)

5. **`retained_messages`**: Retained MQTT messages
   - `topic` (string, primary key)
   - `payload` (bytes, not null)
   - `qos` (byte, not null)
   - `created_at` (timestamp)

**User Architecture:**
- **DashboardUser**: Web UI administrators. Can log in to dashboard and use REST API to manage the system. Role can be 'admin' (full access) or 'viewer' (read-only).
- **MQTTUser**: MQTT credentials that can be shared by multiple devices. Cannot log in to dashboard.
- **MQTTClient**: Individual devices that connect using an MQTTUser's credentials. Tracked with unique Client ID.
- The login endpoint (`POST /api/auth/login`) only accepts DashboardUsers. MQTTUsers authenticate via MQTT protocol.
- Multiple MQTT clients (e.g., sensors in a building) can share the same MQTTUser credentials but have unique Client IDs.

**Database Configuration:**
The server supports three database backends (SQLite, PostgreSQL, MySQL) configured via environment variables or command-line flags:

```bash
# Database Configuration
DB_TYPE=postgres          # Database type: sqlite (default), postgres, mysql
DB_PATH=mqtt.db          # SQLite: file path (default: mqtt-server.db)
DB_HOST=localhost        # Postgres/MySQL: host (default: localhost)
DB_PORT=5432            # Postgres/MySQL: port (default: 5432 for postgres, 3306 for mysql)
DB_USER=mqtt            # Postgres/MySQL: username (default: mqtt)
DB_PASSWORD=secret      # Postgres/MySQL: password
DB_NAME=mqtt            # Postgres/MySQL: database name (default: mqtt)
DB_SSLMODE=disable      # Postgres: SSL mode (default: disable)

# Admin Credentials (ONLY used on first run - like Grafana)
ADMIN_USERNAME=admin     # Default admin username (default: admin)
ADMIN_PASSWORD=admin     # Default admin password (default: admin)

# Command-line flags (override environment variables for database config)
./mqtt-server \
  -db-type postgres \
  -db-host localhost \
  -db-port 5432 \
  -db-user mqtt \
  -db-password secret \
  -db-name mqtt \
  -db-sslmode disable
```

**Docker Compose Examples:**
- `compose.yml` - SQLite (default, embedded database)
- `examples/compose.postgres.yml` - PostgreSQL with separate database container
- `examples/compose.mysql.yml` - MySQL with separate database container

**Logging Configuration:**
The server uses Go's `log/slog` for structured logging with configurable levels and output formats:

```bash
# Logging Configuration (environment variables)
LOG_LEVEL=info          # Log level: debug, info, warn, error (default: info)
LOG_FORMAT=text         # Output format: text, json (default: text)

# Examples:
# Development with debug logging
LOG_LEVEL=debug LOG_FORMAT=text ./mqtt-server

# Production with JSON logging for aggregation tools
LOG_LEVEL=info LOG_FORMAT=json ./mqtt-server
```

Log levels:
- `debug` - Verbose logging including client connections/disconnections, tracking details
- `info` - Standard operational logging (default)
- `warn` - Warning messages (auth failures, errors that don't stop execution)
- `error` - Error messages (ACL check errors, database errors)

Output format examples:
```
# Text format (human-readable, for development)
time=2025-10-23T08:02:38.347-04:00 level=INFO msg="Starting MQTT Server"
time=2025-10-23T08:02:38.347-04:00 level=INFO msg="Connecting to database" type=sqlite

# JSON format (machine-readable, for production log aggregation)
{"time":"2025-10-23T08:02:38.347-04:00","level":"INFO","msg":"Starting MQTT Server"}
{"time":"2025-10-23T08:02:38.347-04:00","level":"INFO","msg":"Connecting to database","type":"sqlite"}
```

**Key functions:**

*Database Management:*
- `storage.Open(config)` - Opens database with GORM, runs AutoMigrate, adds default admin
- `storage.LoadConfigFromEnv()` - Loads configuration from environment variables
- `storage.DefaultSQLiteConfig(path)` - Creates SQLite configuration

*AdminUser (Dashboard) Management:*
- `db.CreateAdminUser(username, password, role)` - Create dashboard admin
- `db.AuthenticateAdminUser(username, password)` - Validate admin login
- `db.GetAdminUser(id)` / `db.GetAdminUserByUsername(username)` - Retrieve admin
- `db.ListAdminUsers()` - List all admin users
- `db.UpdateAdminUser(id, username, role)` - Update admin info
- `db.UpdateAdminUserPassword(id, password)` - Change admin password
- `db.DeleteAdminUser(id)` - Delete admin user

*MQTTUser (Credentials) Management:*
- `db.CreateMQTTUser(username, password, description, metadata)` - Create MQTT credentials
- `db.AuthenticateMQTTUser(username, password)` - Validate MQTT credentials
- `db.AuthenticateUser(username, password)` - Compatibility wrapper for auth hook
- `db.GetMQTTUser(id)` / `db.GetMQTTUserByUsername(username)` - Retrieve MQTT user
- `db.ListMQTTUsers()` - List all MQTT credentials
- `db.UpdateMQTTUser(id, username, description, metadata)` - Update MQTT user
- `db.UpdateMQTTUserPassword(id, password)` - Change MQTT password
- `db.DeleteMQTTUser(id)` - Delete MQTT user (cascades to clients and ACL rules)

*MQTTClient (Device Tracking) Management:*
- `db.UpsertMQTTClient(clientID, mqttUserID, metadata)` - Create/update client record on connect
- `db.MarkMQTTClientInactive(clientID)` - Mark client as disconnected
- `db.GetMQTTClient(id)` / `db.GetMQTTClientByClientID(clientID)` - Retrieve client
- `db.ListMQTTClients(activeOnly)` - List all clients or just active ones
- `db.ListMQTTClientsByUser(mqttUserID, activeOnly)` - List clients for a specific MQTT user
- `db.UpdateMQTTClientMetadata(clientID, metadata)` - Update client metadata
- `db.DeleteMQTTClient(id)` - Delete client record
- `db.GetClientCount(activeOnly)` - Count total or active clients

*ACL Management:*
- `db.CreateACLRule(mqttUserID, topicPattern, permission)` - Create ACL rule for MQTT user
- `db.UpdateACLRule(id, topicPattern, permission)` - Update existing ACL rule
- `db.ListACLRules()` - List all ACL rules
- `db.GetACLRulesByMQTTUserID(mqttUserID)` - Get rules for specific MQTT user
- `db.DeleteACLRule(id)` - Delete ACL rule
- `db.CheckACL(username, clientID, topic, action)` - Check if MQTT user can pub/sub to topic
- Topic matching supports:
  - MQTT wildcards: `+` (single level), `#` (multi-level)
  - Dynamic placeholders: `${username}`, `${clientid}` (replaced at runtime)

**Topic Pattern Examples:**
```
# Static patterns with wildcards
devices/+/telemetry        # Matches devices/sensor1/telemetry, devices/sensor2/telemetry
commands/#                 # Matches all subtopics under commands/

# Dynamic patterns with ${username} placeholder (multi-tenant isolation)
user/${username}/data      # alice can only access user/alice/data
user/${username}/#         # bob can access all subtopics under user/bob/

# Dynamic patterns with ${clientid} placeholder (per-device isolation)
device/${clientid}/status  # client "sensor-001" can only access device/sensor-001/status

# Combined placeholders and wildcards
users/${username}/devices/${clientid}/#  # Full isolation per user and device
```

**Use Cases for Placeholders:**
- **Multi-tenant systems**: Users can only access their own namespace
- **IoT deployments**: Devices can only publish to topics matching their client ID
- **Security**: No need to create individual ACL rules per user/device

**Auto-Migration:**
To add a new column to a table, simply update the struct in `models.go`:
```go
type AdminUser struct {
    ID           uint
    Username     string
    Email        string  // ← Add new field here
    PasswordHash string
    Role         string
    CreatedAt    time.Time
}
```
Restart the app - GORM will automatically add the `email` column!

**Default Admin Credentials:**
- Default: `admin` / `admin` (auto-created on first run)
- Configurable via `ADMIN_USERNAME` and `ADMIN_PASSWORD` environment variables
- **Important:** Like Grafana, these env vars **ONLY work on first launch**
- Once the admin user exists in the database, changing env vars has no effect
- To reset: delete the database/volume and restart, or use the API to change the password
- For production: Set custom credentials before first run to avoid using defaults

### 2. MQTT Hooks

Hooks implement the mochi-mqtt hook interface to intercept MQTT lifecycle events:

**AuthHook** (`hooks/auth/auth.go`):
- Implements `OnConnectAuthenticate` to validate MQTT client credentials against database
- Validates against MQTTUser table (not DashboardUser)
- Anonymous connections allowed if no username provided
- Stores username in `cl.Properties.Username` for ACL checks

**ACLHook** (`hooks/auth/acl.go`):
- Implements `OnACLCheck` to authorize publish/subscribe operations
- Reads username from `cl.Properties.Username`
- Calls `db.CheckACL()` to validate against stored ACL rules
- Checks against MQTTUser credentials and their ACL rules

**TrackingHook** (`hooks/tracking/tracking.go`):
- Implements `OnConnect` to track client connections in the database
- Creates or updates MQTTClient record with first_seen, last_seen, is_active
- Implements `OnDisconnect` to mark clients as inactive
- Automatically tracks which devices are using which MQTT credentials

**MetricsHook** (`hooks/metrics/metrics.go`):
- Tracks Prometheus metrics for connections, messages, bytes transferred
- Updates on connect/disconnect, publish, subscribe events

**RetainedHook** (`hooks/retained/retained.go`):
- Persists retained messages to database
- Automatically loads retained messages on startup
- Implements `StoredRetainedMessages()` and `OnRetainMessage()`

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
- `POST /api/auth/login` - Get JWT token (DashboardUser only)

Protected (any authenticated admin):
- `PUT /api/auth/change-password` - Change own password

Protected (admin only - all routes below):

*Dashboard Administration:*
- `GET /api/admin/users` - List all admin users
- `POST /api/admin/users` - Create admin user
- `PUT /api/admin/users/{id}` - Update admin user
- `PUT /api/admin/users/{id}/password` - Reset admin password
- `DELETE /api/admin/users/{id}` - Delete admin user

*MQTT User Management:*
- `GET /api/mqtt/users` - List all MQTT users
- `POST /api/mqtt/users` - Create MQTT user
- `PUT /api/mqtt/users/{id}` - Update MQTT user
- `PUT /api/mqtt/users/{id}/password` - Reset MQTT user password
- `DELETE /api/mqtt/users/{id}` - Delete MQTT user (cascades to clients and ACL)

*MQTT Clients (Connected Devices):*
- `GET /api/mqtt/clients` - List all MQTT clients (with active status)
- `GET /api/mqtt/clients/{client_id}` - Get client details
- `PUT /api/mqtt/clients/{client_id}/metadata` - Update client metadata
- `DELETE /api/mqtt/clients/{id}` - Delete client record

*ACL Rules:*
- `GET /api/acl` - List all ACL rules
- `POST /api/acl` - Create ACL rule for MQTT user
- `PUT /api/acl/{id}` - Update ACL rule
- `DELETE /api/acl/{id}` - Delete ACL rule

*Bridge Management:*
- `GET /api/bridges` - List all bridges (with pagination)
- `GET /api/bridges/{id}` - Get specific bridge
- `POST /api/bridges` - Create bridge
- `PUT /api/bridges/{id}` - Update bridge
- `DELETE /api/bridges/{id}` - Delete bridge

*Legacy Endpoints (for backward compatibility):*
- `GET /api/clients` - List connected MQTT clients
- `GET /api/clients/{id}` - Get client details
- `POST /api/clients/{id}/disconnect` - Force disconnect client

*Monitoring:*
- `GET /api/metrics` - Get server metrics (JSON)
- `GET /metrics` - Prometheus metrics endpoint (no auth)

**Frontend serving:**
- Root path (`/`) serves embedded SPA from `web/dist`
- Falls back to `index.html` for client-side routing

### 5. Main Entry Point (`main.go`)

Orchestrates startup:
1. Parse CLI flags and load environment variables
2. Open database (SQLite/PostgreSQL/MySQL based on config)
3. Create MQTT server with config
4. Register hooks in order:
   - Auth hook (validates MQTT credentials)
   - ACL hook (checks topic permissions)
   - Metrics hook (Prometheus metrics)
   - Retained message hook (persistent retained messages)
   - Tracking hook (records client connections)
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

# Deploy single binary (19MB stripped, includes all 3 database drivers)
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

**User Separation:**
- DashboardUsers can log in to the web dashboard and REST API to manage the system
- MQTTUsers are credentials for MQTT connections (cannot access dashboard/API)
- MQTTClients are individual devices tracked by their Client ID
- Multiple devices can share the same MQTTUser credentials
- ACL rules are attached to MQTTUsers, not individual clients

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

# First, create MQTT credentials via API (see Testing API section)
# Then use those credentials (NOT admin credentials) for MQTT:

# Test anonymous connection (if enabled)
mosquitto_pub -h localhost -p 1883 -t "test/topic" -m "hello"

# Test with MQTT credentials (requires creating MQTT user via API first)
mosquitto_pub -h localhost -p 1883 -u sensor_user -P sensor123 -t "test/topic" -m "hello"

# Subscribe with MQTT credentials
mosquitto_sub -h localhost -p 1883 -u sensor_user -P sensor123 -t "test/#"

# Test with different client IDs (same credentials, different devices)
mosquitto_pub -h localhost -p 1883 -i "device-001" -u sensor_user -P sensor123 -t "sensor/temp" -m "22.5"
mosquitto_pub -h localhost -p 1883 -i "device-002" -u sensor_user -P sensor123 -t "sensor/temp" -m "23.1"

# Note: DashboardUser credentials (admin/admin) do NOT work for MQTT connections
# You must create separate MQTTUser credentials via the API
```

## Testing API

```bash
# Login as admin
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Save token from response, then:
TOKEN="<your-token>"

# === Dashboard Admin Management ===
# List admin users
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/admin/users

# Create admin user
curl -X POST http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"newadmin","password":"secure123","role":"admin"}'

# === MQTT User Management ===
# Create MQTT user
curl -X POST http://localhost:8080/api/mqtt/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"sensor_user","password":"sensor123","description":"Sensor credentials"}'

# List MQTT users
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/mqtt/users

# === MQTT Clients Tracking ===
# List all connected clients
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/mqtt/clients

# Get specific client details
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/mqtt/clients/sensor-device-001

# Update client metadata
curl -X PUT http://localhost:8080/api/mqtt/clients/sensor-device-001/metadata \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"location":"warehouse-A","device_type":"temperature_sensor"}'

# === ACL Rules ===
# Create ACL rule with wildcards
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mqtt_user_id":1,"topic_pattern":"sensor/+/temp","permission":"pubsub"}'

# Create ACL rule with ${username} placeholder (multi-tenant isolation)
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mqtt_user_id":1,"topic_pattern":"user/${username}/#","permission":"pubsub"}'

# Create ACL rule with ${clientid} placeholder (per-device isolation)
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mqtt_user_id":1,"topic_pattern":"device/${clientid}/status","permission":"pub"}'

# Create ACL rule with combined placeholders
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mqtt_user_id":1,"topic_pattern":"users/${username}/devices/${clientid}/#","permission":"pubsub"}'

# List ACL rules
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/acl

# === Bridge Management ===
# List all bridges
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/bridges

# Get specific bridge
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/bridges/1

# Create bridge
curl -X POST http://localhost:8080/api/bridges \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cloud-bridge",
    "remote_host": "mqtt.example.com",
    "remote_port": 1883,
    "remote_username": "bridge_user",
    "remote_password": "bridge_pass",
    "client_id": "my-bridge-client",
    "clean_session": true,
    "keep_alive": 60,
    "connection_timeout": 30,
    "metadata": {"location": "datacenter-1"},
    "topics": [
      {
        "local_pattern": "sensors/#",
        "remote_pattern": "edge/site-a/sensors/#",
        "direction": "out",
        "qos": 1
      },
      {
        "local_pattern": "commands/#",
        "remote_pattern": "cloud/commands/#",
        "direction": "in",
        "qos": 1
      }
    ]
  }'

# Update bridge
curl -X PUT http://localhost:8080/api/bridges/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cloud-bridge-updated",
    "remote_host": "mqtt-new.example.com",
    "remote_port": 1883,
    "client_id": "my-bridge-client",
    "clean_session": true,
    "keep_alive": 90,
    "connection_timeout": 45,
    "topics": [...]
  }'

# Delete bridge
curl -X DELETE http://localhost:8080/api/bridges/1 \
  -H "Authorization: Bearer $TOKEN"

# Note: Bridges provisioned from config file cannot be modified or deleted via API
# They will return 409 Conflict with an error message

# === Password Management ===
# Change your own password
curl -X PUT http://localhost:8080/api/auth/change-password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"current_password":"admin","new_password":"new_secure_password"}'

# Reset another admin's password
curl -X PUT http://localhost:8080/api/admin/users/2/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"password":"reset_password_123"}'

# Reset MQTT user password
curl -X PUT http://localhost:8080/api/mqtt/users/1/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"password":"new_mqtt_password"}'

# === Monitoring ===
# Get metrics
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/metrics

# Prometheus metrics (no auth)
curl http://localhost:8080/metrics
```

## Security Considerations

- **Set `ADMIN_USERNAME` and `ADMIN_PASSWORD` before first run in production** - these env vars only work on initial startup
- If using default credentials (`admin`/`admin`), change the password immediately via API after first login
- Use TLS for production deployments (not yet implemented)
- Store JWT secret in environment variable, not code
- Implement rate limiting on API endpoints for production
- Consider using proper secrets management for production databases (use Docker secrets or cloud provider secrets)
- Review CORS policy before production deployment
- use slog for golang logging