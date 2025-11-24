# BroMQ

A high-performance, single-node MQTT broker with embedded web UI built on mochi-mqtt/server.

## Why BroMQ?

**Truly open source.** Every feature unlocked, no enterprise tiers, no connection limits. Apache 2.0 licensed, use it however you want.

**Batteries-included, single binary.** Web dashboard, REST API, user management, ACL system, client tracking, MQTT bridging, and Prometheus metrics in one small binary. No plugins required.

**Production-ready for:**

- Self-hosted IoT infrastructure
- Edge-to-cloud architectures via MQTT bridging
- Multi-tenant SaaS with per-user topic isolation
- Kubernetes/Docker deployments
- Development and testing environments

## Comparisons

**vs. Open-core brokers:** No feature paywalls, license pop-ups, or artificial connection limits  
**vs. Mosquitto:** Includes web UI, REST API, and database-backed authentication  
**vs. Cloud platforms:** Self-hosted control, no per-connection pricing  
**vs. Enterprise solutions:** Simpler deployment, no support contracts required

## Feature Comparison

| Feature          | BroMQ                    | EMQX 5.9+     | VerneMQ           | Mosquitto     | HiveMQ        |
| ---------------- | ------------------------ | ------------- | ----------------- | ------------- | ------------- |
| License          | Apache 2.0               | BSL 1.1\*     | Apache 2.0\*\*    | EPL 2.0/EDL   | Commercial    |
| Clustering       | ✅ Bridging              | 💰 Licensed   | ✅ Masterless     | ✅ Bridging   | ⚠️ Enterprise |
| Web Dashboard    | ✅ Built-in              | ✅ Built-in   | ❌ Community only | ❌            | ⚠️ Enterprise |
| REST API         | ✅ Full CRUD             | ✅ Full       | ✅ CLI wrapper    | ❌            | ⚠️ Enterprise |
| Database Auth    | ✅ SQLite/Postgres/MySQL | ✅ Built-in   | ✅ Plugins        | ❌ File-based | ⚠️ Enterprise |
| Connection Limit | ∞ Unlimited              | ∞ Single-node | ∞ Unlimited       | ∞ Unlimited   | 💰 Licensed   |

\*BSL 1.1: Single-node free, clustering requires license, converts to Apache 2.0 after 4 years
\*\*Source code Apache 2.0, official packages/Docker images under EULA

**When BroMQ is the right choice:**

- You want batteries-included (web UI, REST API, auth, ACL) without enterprise licensing
- You want GitOps-friendly declarative YAML config for bridges, users, ACL, and more
- You're deploying to VPS/cloud/edge with bridging support (works as edge OR cloud broker)
- You're deploying to Kubernetes/Docker

**When to consider alternatives:**

- **Massive scale** (>100K connections): EMQX, VerneMQ
- **Ultra-lightweight** (<5MB): Mosquitto
- **Enterprise support contracts**: HiveMQ, EMQX Enterprise

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
- **Web Dashboard:** `http://localhost:8080`
- **Default Login:** `admin` / `admin` ⚠️ Change immediately!

## Configuration Example

BroMQ supports declarative YAML configuration for GitOps workflows:

```yaml
# config.yml - Auto-syncs to database on startup
users:
  - username: sensors
    password: ${MQTT_SENSOR_PASSWORD} # Env var interpolation
    description: "IoT sensor fleet"

  - username: cameras
    password: ${MQTT_CAMERA_PASSWORD}
    description: "Camera devices"

  - username: admin
    password: admin

acl_rules:
  # Multi-tenant isolation with reserved dynamic placeholders
  - username: sensors
    topic: "devices/${username}/#" # Each user isolated
    permission: pubsub

  - username: cameras
    topic: "video/${clientid}/stream" # Per-device topics
    permission: pub

  - username: admin
    topic: "#" # Full access
    permission: pubsub

bridges:
  - name: cloud-bridge
    host: mqtt.example.com
    port: 8883
    topics:
      - local: "data/#"
        remote: "edge/site-1/data/#"
        direction: out

      - local: "commands/#"
        remote: "edge/site-1/commands/#"
        direction: in
```

```bash
# Run with config
export MQTT_SENSOR_PASSWORD="secret123"
export MQTT_CAMERA_PASSWORD="camera456"

# Set custom admin credentials (only used on first run)
export ADMIN_USERNAME="myadmin"
export ADMIN_PASSWORD="securepassword"

docker run \
  -e MQTT_SENSOR_PASSWORD \
  -e MQTT_CAMERA_PASSWORD \
  -e ADMIN_USERNAME \
  -e ADMIN_PASSWORD \
  -v ./config.yml:/app/config.yml \
  bromq -config /app/config.yml
```

**Note:** `ADMIN_USERNAME` and `ADMIN_PASSWORD` environment variables only work on first startup. To change the admin password later, use the web UI or API.

See [examples/config/](examples/config/) for more examples.

### IDE Autocomplete Support

BroMQ provides a JSON Schema for YAML configuration files, enabling IDE autocomplete and validation:

```yaml
# Add this line to the top of your config.yml
# yaml-language-server: $schema=https://github.com/bherbruck/bromq/releases/latest/download/bromq-config.schema.json

users:
  - username: sensor_user  # IDE will show autocomplete here!
    password: ${PASSWORD}
```

**Supported editors:** VS Code, IntelliJ IDEA, WebStorm, PyCharm, and any editor with YAML Language Server support.

**Schema URLs:**
- Latest: `https://github.com/bherbruck/bromq/releases/latest/download/bromq-config.schema.json`
- Version-specific: `https://github.com/bherbruck/bromq/releases/download/v0.0.3/bromq-config.schema.json`

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
