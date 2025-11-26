# CLAUDE.md

Guide for Claude Code when working with BroMQ.

## Project Overview

BroMQ is a production-ready MQTT broker with embedded web UI built on mochi-mqtt/server. Single binary deployment (~19MB) with database-backed authentication, ACL system, MQTT bridging, JavaScript scripting engine, and REST API.

**Core purpose:** Provide a batteries-included, truly open-source MQTT broker for IoT, edge computing, and multi-tenant systems without enterprise licensing restrictions.

## Tech Stack

- **Go 1.25+** - Backend with stdlib net/http
- **mochi-mqtt/server v2** - MQTT broker core
- **GORM** - ORM with auto-migration (SQLite/PostgreSQL/MySQL)
- **BadgerDB** - Embedded key-value store for high-write operational data
- **goja** - JavaScript engine for scripting
- **JWT** - API authentication
- **React Router v7 + shadcn/ui** - Frontend (embedded via go:embed)
- **Prometheus** - Metrics

## Database Architecture

BroMQ uses a **dual-database architecture** optimized for different access patterns:

### RDBMS (SQLite/PostgreSQL/MySQL) - Configuration & Low-Write Data
Stores configuration, authentication, and infrequently-updated data with in-memory caching:
- **Users & Auth**: Dashboard users, MQTT users/credentials, ACL rules
- **Configuration**: Bridges, bridge topics, scripts, script triggers
- **Tracking**: MQTT client metadata (first_seen, last_seen, is_active)
- **Script State**: Persistent key-value store for scripts (legacy, still in RDBMS)

**Why RDBMS?**
- Complex queries (JOINs, foreign keys, transactions)
- ACID guarantees for configuration changes
- Natural fit for relational data (users → ACL rules, scripts → triggers)
- In-memory cache eliminates read performance concerns

### BadgerDB - High-Write Operational Data
Stores append-only, time-series data with heavy write volume:
- **Script Logs**: Execution logs from script runs (all `log.info()`, `log.error()`, etc.)
- **Script State**: Persistent key-value store for script variables (NEW - migrated from RDBMS)
- **Retained Messages**: MQTT retained messages

**Why BadgerDB?**
- **LSM-tree architecture**: Optimized for sequential writes (10-100x faster than SQLite)
- **Zero write contention**: No blocking on concurrent writes (SQLite has single writer)
- **Time-series optimized**: Efficient range scans for time-based queries/cleanup
- **Embedded**: No external dependencies, automatic GC and compaction

**Performance Example:**
- Script logging 1000 events/sec on SQLite → write contention, lock timeouts
- Script logging 1000 events/sec on BadgerDB → smooth, no blocking

## Project Structure

```
bromq/
├── cmd/server/main.go          # Entry point, wires hooks and servers
├── internal/
│   ├── storage/                # RDBMS layer (GORM models + CRUD)
│   ├── badgerstore/            # BadgerDB layer (key-value store)
│   │   ├── badgerstore.go     # Core BadgerDB wrapper
│   │   ├── retained.go        # Retained MQTT messages
│   │   ├── script_state.go    # Script persistent state
│   │   └── script_logs.go     # Script execution logs
│   ├── api/                    # REST API (JWT auth)
│   ├── mqtt/                   # MQTT server wrapper
│   ├── script/                 # JavaScript engine
│   │   ├── engine.go          # Script lifecycle management
│   │   ├── runtime.go         # goja VM execution
│   │   └── api.go             # Script API (mqtt.publish, state.get, log.info, etc)
│   ├── config/                 # YAML config parsing
│   └── provisioning/           # Config-to-DB sync (Grafana-style)
├── hooks/                      # MQTT hooks (mochi-mqtt interface)
│   ├── auth/                   # Authentication + ACL
│   ├── tracking/               # Client connection tracking
│   ├── metrics/                # Prometheus metrics
│   ├── retained/               # Retained messages (uses BadgerDB)
│   ├── bridge/                 # MQTT bridging
│   └── script/                 # Script execution (uses BadgerDB for logs)
└── web/                        # Frontend (React Router v7 SPA)
```

## Database Schema

### RDBMS Tables (SQLite/PostgreSQL/MySQL)

**Three-table user architecture:**
1. **`dashboard_users`** - Web UI administrators (can login to dashboard)
2. **`mqtt_users`** - MQTT credentials (shared by devices, cannot login to dashboard)
3. **`mqtt_clients`** - Individual device tracking (one per Client ID)

**Configuration tables:**
- **`acl_rules`** - Topic permissions per MQTT user
- **`bridges`** + **`bridge_topics`** - MQTT bridge configurations
- **`scripts`** + **`script_triggers`** - JavaScript script definitions
- **`script_state`** - Legacy script state (being migrated to BadgerDB)

### BadgerDB Keys (Embedded Key-Value Store)

**Script operations:**
- `log:{scriptID}:{timestamp_ns}` - Script execution logs (JSON)
- `script:{scriptID}:{key}` - Script-scoped persistent state
- `global:{key}` - Global persistent state (shared across scripts)

**MQTT operations:**
- `retained:{topic}` - Retained MQTT messages (JSON)

**Key design:**
- Timestamp-based keys enable efficient time-series queries
- Prefix-based keys enable efficient range scans and cleanup
- TTL support for automatic expiration

## Key Concepts

### User Separation

- **DashboardUser** - Logs into web UI, manages system via API
- **MQTTUser** - Credentials for MQTT connections (shared by multiple devices)
- **MQTTClient** - Individual tracked device (unique Client ID)
- Multiple devices can use same MQTTUser credentials with different Client IDs

### ACL System

- Rules attached to **MQTTUser**, not individual clients
- Supports MQTT wildcards: `+` (single level), `#` (multi-level)
- Supports dynamic placeholders: `${username}`, `${clientid}`
- Examples:
  - `sensor/+/temp` - Wildcard matching
  - `user/${username}/#` - Multi-tenant isolation
  - `device/${clientid}/status` - Per-device isolation

### Provisioning (Config-as-Code)

- YAML config file syncs to database on startup (Grafana-style)
- **Env var interpolation (Docker Compose style):**
  - `${VAR}` - Expand environment variable
  - `${VAR:-default}` - With default value if unset/empty
  - `${username}`, `${clientid}` - Reserved placeholders (NOT expanded)
  - `$${...}` - Escaped, becomes literal `${...}` (for JavaScript templates)
- Supports: users, ACL rules, bridges, scripts
- Provisioned items marked with `provisioned_from_config=true`
- **Cannot modify/delete via API** (returns 409 Conflict)
- See `examples/config/` for examples

**IDE Autocomplete:**

- JSON Schema available for YAML validation and autocomplete
- Add to top of config file: `# yaml-language-server: $schema=https://github.com/bromq-dev/bromq/releases/latest/download/bromq-config.schema.json`
- Schema URLs:
  - Latest: `https://github.com/bromq-dev/bromq/releases/latest/download/bromq-config.schema.json`
  - Version-specific: `https://github.com/bromq-dev/bromq/releases/download/v{version}/bromq-config.schema.json`
- Auto-generated from Go struct tags before each release
- Supports: VS Code, IntelliJ IDEA, WebStorm, PyCharm, and any YAML Language Server

### MQTT Bridging

- Connect to remote MQTT brokers
- Bidirectional topic routing (in/out/both)
- Topic pattern remapping
- Auto-reconnect with exponential backoff
- Managed via `bridge.Manager`

### JavaScript Scripting

- Execute custom logic on MQTT events (publish, connect, disconnect, subscribe)
- JavaScript engine: goja (pure Go)
- Script API: `msg` (message context), `mqtt.publish()`, `state.get()`, `state.set()`, `log.info()`, `global.get()`
- Configurable timeouts (global + per-script)
- Persistent state storage
- Execution logs with retention

## Common Commands

```bash
# Build everything
make build

# Run locally
make run

# Testing
make test            # Go tests
make test-web        # Frontend tests
make test-all        # All tests

# Clean
make clean           # Remove build artifacts + volumes
```

## Configuration

**Environment Variables:**

```bash
# Database (CLI flags override env vars)
DB_TYPE=sqlite              # sqlite (default), postgres, mysql
DB_PATH=bromq.db           # SQLite path
DB_HOST=localhost          # Postgres/MySQL host
DB_PORT=5432               # Postgres/MySQL port
DB_USER=mqtt               # Postgres/MySQL user
DB_PASSWORD=secret         # Postgres/MySQL password
DB_NAME=mqtt               # Postgres/MySQL database
DB_SSLMODE=disable         # Postgres SSL mode (disable, require, verify-ca, verify-full)

# MQTT Server
MQTT_TCP_ADDR=:1883                # TCP listener address
MQTT_WS_ADDR=:8883                 # WebSocket listener address
MQTT_ENABLE_TLS=false              # Enable TLS
MQTT_TLS_CERT=/path/to/cert.pem    # TLS certificate file
MQTT_TLS_KEY=/path/to/key.pem      # TLS key file
MQTT_MAX_CLIENTS=0                 # Max concurrent clients (0 = unlimited)
MQTT_RETAIN_AVAILABLE=true         # Enable retained messages
MQTT_ALLOW_ANONYMOUS=false         # Allow anonymous connections (insecure)

# HTTP API
HTTP_ADDR=:8080            # HTTP API server address
JWT_SECRET=<secret>        # JWT secret for token signing (auto-generated if not set)

# Admin (ONLY used on first run)
ADMIN_USERNAME=admin       # Default: admin
ADMIN_PASSWORD=admin       # Default: admin

# Logging
LOG_LEVEL=info             # debug, info, warn, error
LOG_FORMAT=text            # text, json

# Scripts
SCRIPT_TIMEOUT=5s                        # Global timeout (100ms-5m)
SCRIPT_MAX_PUBLISHES_PER_EXECUTION=100   # Max publishes per execution (1-10000)
SCRIPT_LOG_RETENTION=30d                 # Log retention period

# Config file
CONFIG_FILE=config.yml     # Path to YAML config
```

**CLI Flags:**

```bash
# All flags have corresponding environment variables
# Use --help to see all available flags with descriptions

./bromq --help              # Show all flags and descriptions
./bromq --version           # Show version and exit

# Example with common flags:
./bromq \
  --db-type postgres \
  --db-host localhost \
  --db-port 5432 \
  --db-user mqtt \
  --db-password secret \
  --mqtt-tcp :1883 \
  --mqtt-ws :8883 \
  --mqtt-allow-anonymous \
  --http :8080 \
  --config config.yml

# Shorthand flags available:
./bromq -c config.yml -v    # -c for --config, -v for --version
```

## API Overview

**Authentication:** JWT tokens (24h expiry) via `POST /api/auth/login`

**Key endpoints:**

- `/api/auth/login` - Login (DashboardUser only)
- `/api/admin/users` - Dashboard admin management
- `/api/mqtt/users` - MQTT credentials CRUD
- `/api/mqtt/clients` - Client tracking
- `/api/acl` - ACL rules
- `/api/bridges` - Bridge management
- `/api/scripts` - Script management
- `/api/scripts/{id}/logs` - Script logs
- `/api/metrics` - Server metrics (JSON, auth required)
- `/metrics` - Prometheus metrics (no auth)

See `internal/api/*_handlers.go` for full API.

## Development Notes

**Adding database columns:**
Update struct in `internal/storage/models.go` - GORM auto-migrates on startup.

**Hook execution order:**

1. Metrics hook (tracks everything)
2. Auth hook (validates credentials)
3. ACL hook (checks permissions)
4. Retained hook (persists messages)
5. Tracking hook (records connections)
6. Bridge hook (forwards messages)
7. Script hook (executes custom logic)

**Security considerations:**

- Set `JWT_SECRET` in production (tokens invalidate on restart if not set)
- Change default `admin`/`admin` credentials immediately
- `ADMIN_USERNAME`/`ADMIN_PASSWORD` only work on first run
- Provisioned items cannot be modified via API (edit config + restart)

**Testing MQTT:**

```bash
# Create MQTT user via API first, then:
mosquitto_pub -h localhost -p 1883 -u sensor_user -P password123 -t "test/topic" -m "hello"
mosquitto_sub -h localhost -p 1883 -u sensor_user -P password123 -t "test/#"
```

**Script development:**

- Scripts execute in sandboxed goja runtime
- Configurable timeouts prevent infinite loops (default 5s)
- Publish rate limit prevents message spam (default 100 per execution)
- Message context: `msg.topic`, `msg.payload`, `msg.clientId`, `msg.username`, `msg.type`
- Logging API: `log.info()`, `log.warn()`, `log.error()`, `log.debug()` (saved to script_logs table)
- State API: `state.get(key)`, `state.set(key, value, {ttl: 3600})`
- Global state API: `global.get(key)`, `global.set(key, value, {ttl: 3600})`
- MQTT API: `mqtt.publish(topic, payload, qos, retain)` - limited to prevent spam

## Architecture Flow

```
Client → MQTT Server (mochi-mqtt)
           ↓
        Hooks (in order):
           → MetricsHook → Prometheus
           → AuthHook → RDBMS (cached)
           → ACLHook → RDBMS (cached)
           → RetainedHook → BadgerDB
           → TrackingHook → RDBMS
           → BridgeHook → Remote MQTT
           → ScriptHook → BadgerDB (logs)
           ↓
     ┌─────────────────────────┐
     │  Dual Storage Layer     │
     ├─────────────────────────┤
     │ RDBMS (GORM + Cache)    │  ← Config, Auth, Metadata
     │ BadgerDB (Key-Value)    │  ← Logs, State, Retained
     └─────────────────────────┘
           ↓
        REST API (JWT auth)
           ↓
        Web UI (React Router v7)
```

## Important Behaviors

- **Provisioned items** (from config file) return 409 Conflict on API modification attempts
- **Default admin** auto-created on first run with `ADMIN_USERNAME`/`ADMIN_PASSWORD`
- **JWT secret** auto-generated if not set (warns in logs, invalidates on restart)
- **Script timeouts** enforced globally + per-script override
- **Bridge reconnection** uses exponential backoff
- **Client tracking** upserts on connect, marks inactive on disconnect
- **ACL wildcards** and **placeholders** evaluated at runtime
- **Retained messages** loaded from DB on startup

## Common Tasks

**Add a new API endpoint:**

1. Add handler function to `internal/api/*_handlers.go`
2. Register route in `internal/api/server.go`
3. Add middleware if needed (auth, admin-only)

**Add a new database table:**

1. Define struct in `internal/storage/models.go`
2. Add to `AutoMigrate()` in `internal/storage/db.go`
3. Add CRUD functions in new file (e.g., `internal/storage/new_table.go`)

**Add a new hook:**

1. Implement hook interface in `hooks/newhook/newhook.go`
2. Register in `main.go` after existing hooks

**Add script API function:**

1. Add function to `internal/script/api.go`
2. Register in `setupScriptAPI()` runtime

**Testing:**

- Go tests: `go test ./...`
- Specific package: `go test ./internal/storage`
- With coverage: `go test -cover ./...`
- Frontend: `cd web && npm test`
