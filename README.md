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

**Development (hot reload for both frontend & backend):**
```bash
# Start dev environment
docker compose -f compose.dev.yml up -d

# Backend:  http://localhost:8080 (auto-reload on Go changes)
# Frontend: http://localhost:5173 (Vite HMR)

# Stop dev environment
docker compose -f compose.dev.yml down
```

### 🔧 Local Development (without Docker)

**Prerequisites:** Go 1.23+, Node 20+

```bash
# Terminal 1: Start backend
go run .

# Terminal 2: Start frontend dev server
cd web && npm run dev

# Visit http://localhost:5173
```

### 🏗️ Manual Build

```bash
# Build frontend
cd web && npm run build && cd ..

# Build Go binary with embedded frontend
go build -o bin/mqtt-server .

# Run
./bin/mqtt-server

# Custom configuration
./bin/mqtt-server -db data/mqtt.db -http :8080 -mqtt-tcp :1883 -mqtt-ws :8883
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

## MQTT Client Testing

```bash
# Subscribe
mosquitto_sub -h localhost -p 1883 -u admin -P admin -t "test/#"

# Publish
mosquitto_pub -h localhost -p 1883 -u admin -P admin -t "test/message" -m "Hello MQTT"
```

## Architecture

- **Backend:** Go with stdlib net/http, SQLite, mochi-mqtt
- **Frontend:** React Router v7 (SPA mode) + shadcn/ui + TailwindCSS 4
- **Authentication:** JWT tokens with bcrypt password hashing
- **ACL:** Topic-level permissions with wildcard support
- **Build:** Single binary with embedded React frontend (17MB)

## Development

### Using Makefile (Easiest)

```bash
make dev-up      # Start dev environment (Docker)
make prod-up     # Start production (Docker)
make logs        # View logs
make clean       # Clean everything
make help        # Show all commands
```

### Manual Development

```bash
# Terminal 1: Start Go backend
go run .

# Terminal 2: Start Vite dev server
cd web && npm run dev
# Visit http://localhost:5173
```

The Vite dev server proxies `/api/*` requests to the Go backend at `:8080`.

### Production Build

```bash
# Build frontend
cd web && npm run build

# Build Go binary with embedded frontend
cd ..
go build -o bin/mqtt-server .

# Run
./bin/mqtt-server
```

## Documentation

- **[CLAUDE.md](./CLAUDE.md)** - Detailed architecture and development guide
- **[DOCKER.md](./DOCKER.md)** - Complete Docker deployment guide
- **Makefile** - Run `make help` for all commands

## License

Apache 2.0
