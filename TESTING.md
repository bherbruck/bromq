# MQTT Bridge Testing Guide

This document describes the testing strategy and coverage for the MQTT bridge functionality.

## Test Coverage

### Unit Tests (`hooks/bridge/topic_test.go`)

**Topic Pattern Matching:**
- ✅ Exact topic matches
- ✅ Single-level wildcards (`+`)
- ✅ Multi-level wildcards (`#`)
- ✅ Combined wildcards
- ✅ Edge cases (empty patterns, mismatched levels)

**Topic Transformation:**
- ✅ Multi-level wildcard transformations
- ✅ Single-level wildcard transformations
- ✅ Exact transformations (no wildcards)
- ✅ Complex patterns with prefixes

**Run unit tests:**
```bash
go test ./hooks/bridge/
```

### Integration Tests (`test/bridge_integration_test.go`)

**End-to-End Bridge Functionality:**
- ✅ **Outbound bridging** - Messages published locally forward to remote broker
- ✅ **Inbound bridging** - Messages from remote broker delivered locally
- ✅ **Bidirectional** - Rapid simultaneous messaging in both directions

**Test Setup:**
- Two test MQTT servers (one local, one acting as remote)
- Real bridge connections using paho MQTT client
- Message verification with timeouts

**Run integration tests:**
```bash
go test -tags=integration ./test/
```

**Run bridge integration tests specifically:**
```bash
go test -tags=integration ./test/ -run TestBridge
```

**Run integration tests in verbose mode:**
```bash
go test -tags=integration -v ./test/
```

### Manual Tests

**Real-World Test with test.mosquitto.org:**

Tested with public MQTT broker to verify:
- ✅ Connection to external broker
- ✅ Topic transformation (e.g., `test/local/#` → `bromq-bridge-test/from-local/#`)
- ✅ Outbound message delivery
- ✅ Inbound message reception
- ✅ Inline client integration

**Test configuration used:**
```yaml
bridges:
  - name: test-bridge
    remote_host: test.mosquitto.org
    remote_port: 1883
    topics:
      - local_pattern: "test/local/#"
        remote_pattern: "bromq-bridge-test/from-local/#"
        direction: out
        qos: 0
      - local_pattern: "test/remote/#"
        remote_pattern: "bromq-bridge-test/to-local/#"
        direction: in
        qos: 0
```

## Running All Tests

```bash
# Run all unit tests
go test ./...

# Run bridge unit tests
go test ./hooks/bridge/

# Run bridge integration tests
go test -tags=integration ./hooks/bridge/

# Run tests with coverage
go test -cover ./hooks/bridge/

# Run tests with race detection
go test -race ./hooks/bridge/

# Run integration tests with timeout
go test -tags=integration -timeout 30s ./hooks/bridge/
```

## Test Results Summary

| Test Type | Status | Coverage |
|-----------|--------|----------|
| Unit Tests - Topic Matching | ✅ PASS | 17 test cases |
| Unit Tests - Topic Transformation | ✅ PASS | 6 test cases |
| Integration - Outbound Bridging | ✅ PASS | Real message forwarding |
| Integration - Inbound Bridging | ✅ PASS | Real message reception |
| Integration - Bidirectional | ✅ PASS | 10 messages each direction |
| Manual - Public Broker | ✅ PASS | test.mosquitto.org |

## Performance Benchmarks

Included benchmarks for critical functions:

```bash
go test -bench=. ./hooks/bridge/
```

**Benchmark results:**
- `BenchmarkMatchTopic` - Topic pattern matching performance
- `BenchmarkTransformTopic` - Topic transformation performance

## Continuous Integration

Add to CI/CD pipeline:

```yaml
# .github/workflows/test.yml
- name: Run unit tests
  run: go test ./...

- name: Run integration tests
  run: go test -tags=integration ./...

- name: Run tests with race detector
  run: go test -race ./...
