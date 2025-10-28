# MQTT Server

A high-performance, single-node MQTT broker with embedded web UI built on mochi-mqtt/server.

## Features

- **Full MQTT v3/v5 support** via mochi-mqtt
- **Multi-database support** - SQLite (default), PostgreSQL, MySQL
- **Three-table architecture** - Separate dashboard users, MQTT credentials, and client tracking
- **Database-backed authentication** with bcrypt password hashing
- **Granular ACL permissions** with MQTT wildcard support (`+`, `#`)
- **MQTT Bridging** - Connect to remote brokers with bidirectional topic routing
- **REST API** for comprehensive management (users, credentials, clients, ACL, bridges)
- **Modern Web Dashboard** - React Router v7 + shadcn/ui
- **Client connection tracking** - Monitor individual devices with metadata
- **Configuration provisioning** - YAML-based configuration with auto-sync
- **Single binary deployment** (~19MB) with embedded frontend
- **Docker support** with multi-stage builds and hot reload dev mode
- **Prometheus metrics** endpoint for monitoring

## Quick Start

### 🐳 Docker Compose (Recommended)

**Production (single binary with embedded UI):**

```bash
# Start the server
docker compose up -d

# View logs
docker compose logs -f

# Stop the server
docker compose down
```

### 📍 Access Points

After starting:

- **MQTT TCP:** `localhost:1883`
- **MQTT WebSocket:** `localhost:8883`
- **Web Dashboard:** `http://localhost:8080` (or `:5173` in dev)
- **Default Login:** `admin` / `admin` ⚠️ Change immediately!

## API Usage

### Login (Dashboard User)

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
```

### Create MQTT Credentials

```bash
curl -X POST http://localhost:8080/api/mqtt/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"sensor_user","password":"sensor123","description":"IoT sensors"}'
```

### Create ACL Rule

```bash
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mqtt_user_id":1,"topic_pattern":"sensor/+/temp","permission":"pubsub"}'
```

### List Connected Clients

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/mqtt/clients
```

See [CLAUDE.md](CLAUDE.md) for comprehensive API documentation.

## Architecture

- **Backend:** Go 1.22+ with stdlib net/http, GORM, mochi-mqtt/server v2
- **Database:** SQLite (default), PostgreSQL, or MySQL with auto-migration
- **Frontend:** React Router v7 (SPA mode) + shadcn/ui + Tailwind CSS
- **Authentication:** JWT tokens with bcrypt password hashing
- **User System:** Three-table architecture
  - `dashboard_users` - Web UI administrators
  - `mqtt_users` - MQTT credentials (shared by devices)
  - `mqtt_clients` - Individual device tracking
- **ACL:** Topic-level permissions with MQTT wildcard support (`+`, `#`)
- **Hooks:** Authentication, ACL, client tracking, metrics, retained messages
- **Deployment:** Single binary (~19MB) with embedded frontend

For detailed architecture documentation, see [CLAUDE.md](CLAUDE.md).

## License

Apache 2.0
