# MQTT Server

A high-performance, single-node MQTT broker with embedded web UI built on mochi-mqtt/server.

## Features

- **Full MQTT v3/v5 support** via mochi-mqtt
- **Database-backed authentication** using SQLite
- **Granular ACL permissions** with MQTT wildcard support (`+`, `#`)
- **REST API** for user and ACL management
- **Web Dashboard** (Vite + shadcn/ui - to be scaffolded)
- **Single binary deployment** with embedded frontend
- **Docker support** with multi-stage builds

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

### Login

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
```

### Create User

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"pass123","role":"user"}'
```

### Create ACL Rule

```bash
curl -X POST http://localhost:8080/api/acl \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id":2,"topic_pattern":"sensor/+/temp","permission":"pubsub"}'
```

## Architecture

- **Backend:** Go with stdlib net/http, SQLite, mochi-mqtt
- **Frontend:** React Router v7 (SPA mode) + shadcn/ui + TailwindCSS 4
- **Authentication:** JWT tokens with bcrypt password hashing
- **ACL:** Topic-level permissions with wildcard support
- **Build:** Single binary with embedded React frontend (17MB)

## License

Apache 2.0
