# Fix Bridge Connection Stability and Add Loop Prevention

## Summary

This PR fixes critical bridge connection issues and adds loop prevention for bidirectional MQTT bridges. Includes working hub-and-spoke example configurations with Docker Compose setup.

## Problems Solved

### 1. Bridge Connections Dropping After ~60 Seconds

**Issue:** Bridges would connect successfully but drop after keep-alive timeout with errors:
```
Bridge connection lost: write tcp ... use of closed network connection
```

**Root Cause:** AutoReconnect was disabled, preventing Paho MQTT client from sending keep-alive PINGREQ packets. Remote brokers would close idle connections after the keep-alive timeout.

**Fix:** Enable Paho's built-in AutoReconnect mechanism which handles both keep-alive pings and reconnection with exponential backoff.

### 2. Infinite Message Loops in Bidirectional Bridges

**Issue:** Bidirectional bridges (`direction: both`) created infinite message loops:
```
spoke → hub → spoke → hub → ...
```

**Root Cause:** Messages received from remote brokers were forwarded back out, creating loops.

**Fix:**
- Automatically prefix all bridge client IDs with `bridge-`
- Skip forwarding in OnPublish hook when message originates from a bridge client
- Create inline client on local server to represent bridge for proper client ID tracking

### 3. Client ID Collisions

**Issue:** Multiple spokes connecting to same hub with identically named bridges would fight over the same client ID, causing constant disconnects.

**Fix:** Add 8-character random hex suffix to all bridge client IDs (e.g., `bridge-a3f27b8c`)

## Changes

### Bridge Manager (`hooks/bridge/manager.go`)

- **Enable AutoReconnect:** `SetAutoReconnect(true)` with `SetMaxReconnectInterval(time.Minute)` and `SetResumeSubs(true)`
- **Auto-prefix client IDs:** Add `bridge-{random}` prefix for loop prevention and collision avoidance
- **Inline client creation:** Create virtual client on local server for InjectPacket to use correct client ID
- **Use InjectPacket:** Replace `server.Publish()` with `InjectPacket()` for inbound messages

### Bridge Hook (`hooks/bridge/bridge_hook.go`)

- **Loop prevention:** Skip forwarding when `cl.ID` starts with `bridge-` prefix
- Messages from remote brokers are published locally but not forwarded back out

### Examples

- **Hub configuration:** `examples/config/bridge/hub.yml`
- **Spoke configuration:** `examples/config/bridge/spoke.yml`
- **Docker Compose:** `examples/compose.bridge.yml` with 1 hub + 2 spoke setup

## Testing

**Before:**
- ❌ Connections drop after 60 seconds
- ❌ Infinite message loops with bidirectional bridges
- ❌ Client ID collisions with multiple spokes

**After:**
- ✅ Stable connections with automatic keep-alive
- ✅ No message loops with bidirectional bridges
- ✅ Each bridge gets unique client ID

## Known Limitations

**Duplicate Local Messages:** With bidirectional bridges, messages published locally are delivered twice:
1. First from original publish
2. Second when received back from hub

This is expected MQTT bridge behavior. Solutions:
- Use unidirectional bridges (`direction: out` or `direction: in`)
- Use separate topic namespaces per spoke
- Future: Migrate to MQTT 5 with No Local subscription option

## Future Enhancements

- [ ] Migrate to `paho.golang` for MQTT 5 support
- [ ] Use No Local subscription option to prevent duplicate messages
- [ ] Add protocol version configuration option
- [ ] Add hop count tracking for complex topologies

## Related Issues

Fixes bridge connection stability and enables production-ready hub-and-spoke MQTT architectures.
