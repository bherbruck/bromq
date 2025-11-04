# BroMQ Script Examples

This directory contains example JavaScript scripts that demonstrate various use cases for the BroMQ script system.

## Overview

Scripts execute automatically in response to MQTT events (publish, connect, disconnect, subscribe) and can:
- Transform and route messages
- Monitor conditions and send alerts
- Track statistics and metrics
- Rate limit clients
- Store persistent state

## Example Scripts

### message-logger.js
Simple logging of all published messages with metadata.

**Use case:** Debugging, audit trails

**Trigger:** `on_publish` with topic filter `#` (all topics)

### temperature-alert.js
Monitors temperature readings and sends alerts when threshold exceeded. Uses state to prevent alert flooding.

**Use case:** IoT monitoring, threshold alerts

**Trigger:** `on_publish` with topic filter `sensors/+/temperature`

**Features:**
- Rate limiting (one alert per 5 minutes per sensor)
- Temperature history tracking (last 10 readings)
- State with TTL for automatic cleanup

### connection-tracker.js
Tracks client connection durations and session history.

**Use case:** Connection monitoring, session analytics

**Triggers:**
- `on_connect` to record connection time
- `on_disconnect` to calculate duration

**Features:**
- Global connection counter
- Per-client session history (last 20 sessions)

### rate-limiter.js
Prevents message flooding by counting messages per client.

**Use case:** Abuse prevention, resource protection

**Trigger:** `on_publish` with topic filter `#` (all topics)

**Features:**
- Configurable rate limit (100 messages/minute)
- Automatic reset using TTL
- Logging without blocking (blocking TBD)

### message-router.js
Transforms and republishes messages to different topics.

**Use case:** Protocol translation, message broadcasting

**Trigger:** `on_publish` with topic filter `#` or specific patterns

**Features:**
- Legacy format transformation (CSV → JSON)
- Topic remapping
- Broadcast distribution

### stats-aggregator.js
Aggregates message statistics across all topics using global state.

**Use case:** Metrics collection, usage tracking

**Trigger:** `on_publish` with topic filter `#` (all topics)

**Features:**
- Total message counter
- Per-topic-prefix counters
- Per-client counters
- Periodic stats publication

## JavaScript API

### Event Object

Scripts execute with a `msg` object in scope containing:

```javascript
// For on_publish and on_subscribe
msg.type       // 'publish', 'connect', 'disconnect', 'subscribe'
msg.topic      // MQTT topic
msg.payload    // Message payload (string)
msg.clientId   // Client ID
msg.username   // Username (may be empty)
msg.qos        // Quality of Service (0, 1, 2)
msg.retain     // Retain flag (boolean)

// For on_connect
msg.cleanSession  // Clean session flag

// For on_disconnect
msg.error      // Error message (if abnormal disconnect)
```

### Logging

```javascript
log.debug('Debug message')
log.info('Info message')
log.warn('Warning message')
log.error('Error message')
```

Logs are stored in the database and viewable via API.

### MQTT Operations

```javascript
// Publish a message
mqtt.publish(topic, payload, qos, retain)

// Example
mqtt.publish('alerts/temp', '25.5', 1, false)
```

### State Management

**Script-scoped state** (isolated per script):

```javascript
// Store value
state.set('key', value)

// Store with expiration (TTL in seconds)
state.set('key', value, {ttl: 3600})

// Retrieve value
const value = state.get('key')  // Returns undefined if not found

// Delete value
state.delete('key')

// List all keys
const keys = state.keys()  // Returns array
```

**Global state** (shared across all scripts):

```javascript
// Same API as state, but shared
global.set('key', value)
global.get('key')
global.delete('key')
global.keys()
```

State is:
- **Persistent** - Survives restarts (flushed every 5s + on shutdown)
- **In-memory** - Fast reads/writes
- **Expirable** - Automatic cleanup with TTL

## Log Retention

Script execution logs are automatically cleaned up based on retention settings:

- **Default retention**: 30 days
- **Configurable via env var**: `SCRIPT_LOG_RETENTION`
- **Supported formats**: Standard Go durations plus days (e.g., `30d`, `7d`, `24h`, `1h30m`)
- **Disable cleanup**: Set to `0` to keep logs forever

**Cleanup behavior:**
- Runs automatically in the background
- Check interval is 1/10th of retention period (min 1h, max 24h)
- Examples:
  - `30d` retention → checks daily
  - `7d` retention → checks every 16.8h
  - `24h` retention → checks every 2.4h

**Configuration:**
```bash
# Keep logs for 7 days (check every ~17 hours)
SCRIPT_LOG_RETENTION=7d ./bromq

# Keep logs for 90 days (check daily)
SCRIPT_LOG_RETENTION=90d ./bromq

# Keep logs for 6 hours (check hourly)
SCRIPT_LOG_RETENTION=6h ./bromq

# Disable cleanup (keep forever)
SCRIPT_LOG_RETENTION=0 ./bromq
```

Logs older than the retention period are automatically deleted by a background worker.

## Configuration

Scripts can be configured via:

1. **API** - Create/update scripts via REST API
2. **Config file** - Provision scripts from YAML

### Config File Example

```yaml
scripts:
  - name: message-logger
    description: "Log all messages"
    enabled: true
    script_file: /etc/bromq/scripts/message-logger.js
    triggers:
      - trigger_type: on_publish
        topic_filter: "#"
        priority: 100
        enabled: true

  - name: temp-alert
    description: "Temperature monitoring"
    enabled: true
    script_content: |
      const temp = parseFloat(msg.payload);
      if (temp > 30) {
        mqtt.publish('alerts/temp', 'HIGH: ' + temp, 1, false);
      }
    triggers:
      - trigger_type: on_publish
        topic_filter: "sensors/+/temperature"
        priority: 50
        enabled: true
```

## Topic Pattern Matching

Scripts can filter which events they process using MQTT wildcard patterns:

- `#` - Multi-level wildcard (matches everything)
- `+` - Single-level wildcard
- `sensors/+/temperature` - Matches `sensors/room1/temperature`, `sensors/room2/temperature`
- `devices/#` - Matches all topics under `devices/`

## Execution Order

Multiple scripts can handle the same trigger. Execution order is determined by the `priority` field:

- Lower priority = Earlier execution
- Default priority = 100
- Example: priority 50 runs before priority 100

## Best Practices

1. **Keep scripts simple** - Complex logic should be in your application
2. **Use state wisely** - Don't store large amounts of data
3. **Set TTLs** - Use expiration to prevent unbounded growth
4. **Handle errors** - Wrap risky code in try/catch
5. **Log appropriately** - Use correct log levels (debug/info/warn/error)
6. **Test first** - Use the `/api/scripts/test` endpoint before deploying

## Testing

Test scripts via API before deploying:

```bash
curl -X POST http://localhost:8080/api/scripts/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "script_content": "log.info(\"Test:\", msg.payload)",
    "trigger_type": "on_publish",
    "event_data": {
      "topic": "test/topic",
      "payload": "hello",
      "clientId": "test-client",
      "username": "test-user"
    }
  }'
```

## Limitations

- **Language**: JavaScript only (ES5.1+)
- **Timeout**: 1 second execution limit
- **No async/await**: Synchronous execution only
- **No imports**: Can't require external modules
- **Trusted scripts**: Minimal sandboxing (admin-managed)

## Performance Notes

- Scripts execute **asynchronously** - They don't block message flow
- State reads are **in-memory fast**
- State writes are **batched** (5s flush interval)
- Use **topic filters** to reduce unnecessary executions
- Monitor execution time via logs and API

## Security

- Scripts can only be created/modified by **admin users**
- Scripts provisioned from config file **cannot be modified via API**
- Scripts have access to:
  - MQTT publish operations
  - State storage
  - Logging
- Scripts **cannot**:
  - Access filesystem
  - Make HTTP requests
  - Execute system commands
  - Access other sensitive resources

## Troubleshooting

**Script not executing:**
- Check if script is enabled
- Check trigger type matches event
- Check topic filter matches
- View logs via API: `GET /api/scripts/{id}/logs`

**State not persisting:**
- State flushes every 5 seconds + on shutdown
- Check logs for flush errors
- Ensure graceful shutdown (SIGTERM, not SIGKILL)

**Timeout errors:**
- Scripts have 1 second timeout
- Move heavy processing to your application
- Use async execution patterns

## More Information

- See main `README.md` for API documentation
- See `examples/config/config.yml` for configuration examples
- API docs: `GET /api/scripts` endpoints
