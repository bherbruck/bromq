# CLAUDE.md

Guide for Claude Code when working with BroMQ.

## Project Overview

BroMQ is a production-ready MQTT broker with embedded web UI built on mochi-mqtt/server. Single binary deployment (~19MB) with database-backed authentication, ACL system, MQTT bridging, JavaScript scripting engine, and REST API.

**Core purpose:** Provide a batteries-included, truly open-source MQTT broker for IoT, edge computing, and multi-tenant systems without enterprise licensing restrictions.

## Tech Stack

- **Go 1.25+** - Backend with stdlib net/http
- **mochi-mqtt/server v2** - MQTT broker core
- **GORM** - ORM with auto-migration (SQLite/PostgreSQL/MySQL)
- **goja** - JavaScript engine for scripting
- **JWT** - API authentication
- **React Router v7 + shadcn/ui** - Frontend (embedded via go:embed)
- **Prometheus** - Metrics

## Project Structure

```
bromq/
├── main.go                      # Entry point, wires hooks and servers
├── internal/
│   ├── storage/                 # Database layer (GORM models + CRUD)
│   │   ├── models.go           # Schema: dashboard_users, mqtt_users, mqtt_clients, acl_rules, bridges, scripts, etc
│   │   ├── db.go               # Connection + auto-migration
│   │   ├── config.go           # DB config (env vars + CLI flags)
│   │   ├── dashboard_users.go  # Dashboard admin CRUD
│   │   ├── mqtt_users.go       # MQTT credentials CRUD
│   │   ├── mqtt_clients.go     # Client tracking CRUD
│   │   ├── acl.go              # ACL rules + topic matching
│   │   ├── bridges.go          # Bridge config CRUD
│   │   ├── scripts.go          # Script CRUD
│   │   ├── script_state.go     # Persistent key-value store for scripts
│   │   ├── script_logs.go      # Script execution logs
│   │   └── retained.go         # Retained message persistence
│   ├── api/                    # REST API (JWT auth)
│   │   ├── server.go           # HTTP server + routing
│   │   ├── middleware.go       # JWT validation, CORS, admin guard
│   │   ├── dashboard_handlers.go  # Dashboard user management
│   │   ├── mqtt_handlers.go    # MQTT users + clients + ACL
│   │   ├── bridge_handlers.go  # Bridge management
│   │   ├── script_handlers.go  # Script management + logs
│   │   └── handlers.go         # Metrics + legacy endpoints
│   ├── mqtt/                   # MQTT server wrapper
│   │   ├── server.go           # mochi-mqtt wrapper
│   │   ├── config.go           # Server config
│   │   ├── metrics.go          # Stats extraction
│   │   └── prometheus_metrics.go  # Prometheus metrics
│   ├── config/                 # YAML config parsing
│   │   └── config.go           # Env var interpolation
│   ├── provisioning/           # Config-to-DB sync (Grafana-style)
│   │   └── provisioning.go     # Sync users, ACL, bridges, scripts
│   └── script/                 # JavaScript engine
│       ├── engine.go           # Script lifecycle management
│       ├── runtime.go          # goja VM execution
│       ├── api.go              # Script API (mqtt.publish, state.get, etc)
│       └── state.go            # Persistent state management
├── hooks/                      # MQTT hooks (mochi-mqtt interface)
│   ├── auth/                   # Authentication + ACL
│   │   ├── auth.go            # Validate MQTT credentials
│   │   └── acl.go             # Topic permission checks
│   ├── tracking/              # Client connection tracking
│   │   └── tracking.go        # Track connect/disconnect in DB
│   ├── metrics/               # Prometheus metrics
│   │   └── metrics.go         # MQTT metrics collection
│   ├── retained/              # Retained messages
│   │   └── retained.go        # Persist retained messages to DB
│   ├── bridge/                # MQTT bridging
│   │   ├── manager.go         # Bridge lifecycle management
│   │   ├── bridge_hook.go     # Message forwarding hook
│   │   └── topic.go           # Topic pattern matching
│   └── script/                # Script execution
│       └── script_hook.go     # Execute scripts on MQTT events
├── web/                       # Frontend (React Router v7 SPA)
│   ├── app/                   # React source
│   ├── dist/client/           # Build output (embedded)
│   └── embed.go               # go:embed directive
└── examples/
    └── config/                # YAML config examples
        ├── config.yml         # Full featured example
        ├── minimal.yml        # Minimal example
        └── multitenant.yml    # Multi-tenant isolation example
```

## Database Schema

**Three-table user architecture:**

1. **`dashboard_users`** - Web UI administrators (can login to dashboard)

   - `id`, `username`, `password_hash`, `role` (admin/viewer), `metadata`

2. **`mqtt_users`** - MQTT credentials (shared by devices, cannot login to dashboard)

   - `id`, `username`, `password_hash`, `description`, `metadata`, `provisioned_from_config`

3. **`mqtt_clients`** - Individual device tracking (one per Client ID)
   - `id`, `client_id`, `mqtt_user_id` (FK), `metadata`, `first_seen`, `last_seen`, `is_active`

**Other tables:**

- **`acl_rules`** - Topic permissions (`mqtt_user_id`, `topic_pattern`, `permission`)
- **`bridges`** - Bridge configs (`name`, `remote_host`, `remote_port`, auth, timeouts)
- **`bridge_topics`** - Topic mappings (`bridge_id`, `local_pattern`, `remote_pattern`, `direction`)
- **`scripts`** - JavaScript scripts (`name`, `script_content`, `enabled`, `timeout_seconds`)
- **`script_triggers`** - When to run scripts (`script_id`, `trigger_type`, `topic_filter`)
- **`script_state`** - Persistent key-value store for scripts
- **`script_logs`** - Execution logs
- **`retained_messages`** - Retained MQTT messages

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
- Env var interpolation: `${VAR_NAME}`
- Supports: users, ACL rules, bridges, scripts
- Provisioned items marked with `provisioned_from_config=true`
- **Cannot modify/delete via API** (returns 409 Conflict)
- See `examples/config/` for examples

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

# Development (hot reload)
make dev-up          # Backend :8080, Frontend :5173
make logs            # View logs
make dev-down

# Production
make prod-up         # Single binary with embedded UI
make prod-down

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

# Admin (ONLY used on first run)
ADMIN_USERNAME=admin       # Default: admin
ADMIN_PASSWORD=admin       # Default: admin

# Security
JWT_SECRET=<secret>        # REQUIRED for production (openssl rand -hex 32)

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
./bromq \
  -db-type postgres \
  -db-host localhost \
  -db-port 5432 \
  -db-user mqtt \
  -db-password secret \
  -mqtt-tcp :1883 \
  -mqtt-ws :8883 \
  -http :8080 \
  -config config.yml
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
           → MetricsHook (Prometheus)
           → AuthHook (validate credentials)
           → ACLHook (check topic permissions)
           → RetainedHook (persist retained messages)
           → TrackingHook (record connections)
           → BridgeHook (forward to remote brokers)
           → ScriptHook (execute custom logic)
           ↓
        Database (GORM)
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
