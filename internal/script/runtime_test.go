package script

import (
	"context"
	"strings"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/prometheus/client_golang/prometheus"

	"github/bromq-dev/bromq/internal/badgerstore"
	"github/bromq-dev/bromq/internal/storage"
)

func setupTestRuntime(t *testing.T) (*storage.DB, *badgerstore.BadgerStore, *Runtime, *mqtt.Server) {
	t.Helper()

	// Setup in-memory database
	config := storage.DefaultSQLiteConfig(":memory:")
	cache := storage.NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := storage.OpenWithCache(config, cache)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Verify schema is ready
	ensureScriptTablesExist(t, db)

	// Setup MQTT server with InlineClient enabled for script publishing
	mqttServer := mqtt.New(&mqtt.Options{
		InlineClient: true, // Required for scripts to publish messages
	})
	if err := mqttServer.Serve(); err != nil {
		t.Fatalf("failed to start MQTT server: %v", err)
	}

	// Setup BadgerDB for state and logs
	badger := badgerstore.OpenInMemory(t)

	// Setup state manager and runtime
	stateManager := NewStateManagerBadger(badger)
	runtime := NewRuntime(db, badger, stateManager, mqttServer)

	return db, badger, runtime, mqttServer
}

// ensureScriptTablesExist verifies that script-related tables exist in the database
func ensureScriptTablesExist(t *testing.T, db *storage.DB) {
	t.Helper()

	// Note: script_logs table is no longer required - logs are now in BadgerDB

	// Also verify other script tables (except script_logs which is now in BadgerDB)
	requiredTables := []string{"scripts", "script_triggers", "script_state"}
	for _, tableName := range requiredTables {
		var exists int64
		err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&exists).Error
		if err != nil || exists == 0 {
			t.Fatalf("required table %s does not exist (err: %v)", tableName, err)
		}
	}
}

func TestRuntimeExecuteSuccess(t *testing.T) {
	db, badger, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Create script in database first (needed for foreign key)
	scriptRecord, err := db.CreateScript("test-script", "", `log.info("Hello from script");`, true, []byte("{}"), []storage.ScriptTrigger{})
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	script := scriptRecord

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test payload",
		ClientID: "test-client",
		Username: "test-user",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %v", result.Error)
	}

	if result.ExecutionTimeMs < 0 {
		t.Error("Expected non-negative execution time")
	}

	// Check that user log was created in BadgerDB
	// Note: Success executions no longer auto-log, only user log.* calls are saved
	_, total, err := badger.ListScriptLogs(script.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if total != 1 { // Only user log (no automatic success log)
		t.Errorf("Expected 1 user log, got %d", total)
	}
}

func TestRuntimeExecuteWithError(t *testing.T) {
	db, badger, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Create script in database first (needed for foreign key)
	scriptRecord, err := db.CreateScript("error-script", "", `throw new Error("Test error");`, true, []byte("{}"), []storage.ScriptTrigger{})
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	script := scriptRecord

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if result.Success {
		t.Error("Expected execution to fail, but it succeeded")
	}

	if result.Error == nil {
		t.Error("Expected error to be set")
	}

	// Verify error was logged
	logs, total, _ := badger.ListScriptLogs(script.ID, 1, 10, "")
	if total == 0 {
		t.Error("Expected error log to be created")
	}

	if logs[0].Level != "error" {
		t.Errorf("Expected error level log, got %s", logs[0].Level)
	}
}

func TestRuntimeExecuteTimeout(t *testing.T) {
	_, _, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Set short timeout for testing
	runtime.SetDefaultTimeout(100 * time.Millisecond)

	script := &storage.Script{
		ID:   1,
		Name: "timeout-script",
		Content: `
			var start = Date.now();
			while (Date.now() - start < 500) {
				// Infinite loop that should timeout
			}
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if result.Success {
		t.Error("Expected execution to timeout")
	}

	if result.Error == nil || result.Error.Error() != "execution timeout after 100ms" {
		t.Errorf("Expected timeout error, got: %v", result.Error)
	}
}

func TestRuntimeExecuteWithPanic(t *testing.T) {
	_, _, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:      1,
		Name:    "panic-script",
		Content: `var x = undefined.property;`, // This will cause a panic
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if result.Success {
		t.Error("Expected execution to fail")
	}

	if result.Error == nil {
		t.Error("Expected error to be set after panic")
	}
}

func TestRuntimeExecuteWithEventData(t *testing.T) {
	_, _, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "event-test",
		Content: `
			if (msg.type !== "publish") throw new Error("Wrong type");
			if (msg.topic !== "test/topic") throw new Error("Wrong topic");
			if (msg.payload !== "hello") throw new Error("Wrong payload");
			if (msg.clientId !== "client-123") throw new Error("Wrong clientId");
			if (msg.username !== "user-456") throw new Error("Wrong username");
			if (msg.qos !== 1) throw new Error("Wrong QoS");
			if (msg.retain !== true) throw new Error("Wrong retain");
			log.info("All msg fields correct");
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "hello",
		ClientID: "client-123",
		Username: "user-456",
		QoS:      1,
		Retain:   true,
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
}

func TestRuntimeLogLevels(t *testing.T) {
	_, _, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "log-levels",
		Content: `
			log.debug("Debug message");
			log.info("Info message");
			log.warn("Warn message");
			log.error("Error message");
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	// Check that all 4 log levels were recorded
	if len(result.Logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(result.Logs))
	}

	expectedLevels := []string{"debug", "info", "warn", "error"}
	for i, expectedLevel := range expectedLevels {
		if i < len(result.Logs) && result.Logs[i].Level != expectedLevel {
			t.Errorf("Log %d: expected level %s, got %s", i, expectedLevel, result.Logs[i].Level)
		}
	}
}

func TestRuntimeCompilationError(t *testing.T) {
	_, _, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:      1,
		Name:    "syntax-error",
		Content: `var x = ;`, // Invalid syntax
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if result.Success {
		t.Error("Expected compilation to fail")
	}

	if result.Error == nil || result.Error.Error()[:18] != "compilation error:" {
		t.Errorf("Expected compilation error, got: %v", result.Error)
	}
}

func TestRuntimeExecuteInfiniteLoopWithPublish(t *testing.T) {
	db, _, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Set short timeout for testing
	runtime.SetDefaultTimeout(200 * time.Millisecond)

	// Create script in database (needed for mqtt.publish to work)
	scriptRecord, err := db.CreateScript("infinite-publish", "", `
		while (true) {
			mqtt.publish(msg.topic + "/echo", msg.payload, msg.qos, msg.retain);
		}
	`, true, []byte("{}"), []storage.ScriptTrigger{})
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
		QoS:      0,
		Retain:   false,
	}

	ctx := context.Background()
	startTime := time.Now()
	result := runtime.Execute(ctx, scriptRecord, message)
	duration := time.Since(startTime)

	// Should fail (either timeout or rate limit - both are valid protections)
	if result.Success {
		t.Error("Expected execution to fail (timeout or rate limit)")
	}

	if result.Error == nil {
		t.Error("Expected error (timeout or rate limit)")
	}

	// Verify that execution was actually stopped (not running forever)
	// With rate limit, it stops much faster than timeout
	if duration > 300*time.Millisecond {
		t.Errorf("Execution took too long (%v), infinite loop not interrupted", duration)
	}

	errorMsg := result.Error.Error()
	if strings.Contains(errorMsg, "rate limit") {
		t.Logf("✓ Infinite loop stopped by rate limit after %v: %v", duration, result.Error)
	} else if strings.Contains(errorMsg, "timeout") {
		t.Logf("✓ Infinite loop stopped by timeout after %v: %v", duration, result.Error)
	} else {
		t.Errorf("Unexpected error type: %v", result.Error)
	}
}
