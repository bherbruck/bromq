package script

import (
	"github.com/prometheus/client_golang/prometheus"
	"context"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"

	"github/bherbruck/bromq/internal/storage"
)

func setupTestRuntime(t *testing.T) (*storage.DB, *Runtime, *mqtt.Server) {
	t.Helper()

	// Setup in-memory database
	config := storage.DefaultSQLiteConfig(":memory:")
	cache := storage.NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := storage.OpenWithCache(config, cache)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Verify schema is ready (script_logs table exists)
	ensureScriptTablesExist(t, db)

	// Setup MQTT server with InlineClient enabled for script publishing
	mqttServer := mqtt.New(&mqtt.Options{
		InlineClient: true, // Required for scripts to publish messages
	})
	if err := mqttServer.Serve(); err != nil {
		t.Fatalf("failed to start MQTT server: %v", err)
	}

	// Setup state manager and runtime
	stateManager := NewStateManager(db)
	runtime := NewRuntime(db, stateManager, mqttServer)

	return db, runtime, mqttServer
}

// ensureScriptTablesExist verifies that script-related tables exist in the database
func ensureScriptTablesExist(t *testing.T, db *storage.DB) {
	t.Helper()

	// Check if script_logs table exists
	var count int64
	err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='script_logs'").Scan(&count).Error
	if err != nil {
		t.Fatalf("failed to check for script_logs table: %v", err)
	}

	if count == 0 {
		t.Fatal("script_logs table does not exist - database migration failed")
	}

	// Also verify other script tables
	requiredTables := []string{"scripts", "script_triggers", "script_logs", "script_state"}
	for _, tableName := range requiredTables {
		var exists int64
		err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&exists).Error
		if err != nil || exists == 0 {
			t.Fatalf("required table %s does not exist (err: %v)", tableName, err)
		}
	}
}

func TestRuntimeExecuteSuccess(t *testing.T) {
	db, runtime, mqttServer := setupTestRuntime(t)
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

	// Check that user log was created in database
	// Note: Success executions no longer auto-log, only user log.* calls are saved
	_, total, err := db.ListScriptLogs(script.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if total != 1 { // Only user log (no automatic success log)
		t.Errorf("Expected 1 user log, got %d", total)
	}
}

func TestRuntimeExecuteWithError(t *testing.T) {
	db, runtime, mqttServer := setupTestRuntime(t)
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
	logs, total, _ := db.ListScriptLogs(script.ID, 1, 10, "")
	if total == 0 {
		t.Error("Expected error log to be created")
	}

	if logs[0].Level != "error" {
		t.Errorf("Expected error level log, got %s", logs[0].Level)
	}
}

func TestRuntimeExecuteTimeout(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Set short timeout for testing
	runtime.SetDefaultTimeout(100 * time.Millisecond)

	script := &storage.Script{
		ID:   1,
		Name: "timeout-script",
		ScriptContent: `
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
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:            1,
		Name:          "panic-script",
		ScriptContent: `var x = undefined.property;`, // This will cause a panic
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
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "event-test",
		ScriptContent: `
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
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "log-levels",
		ScriptContent: `
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
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:            1,
		Name:          "syntax-error",
		ScriptContent: `var x = ;`, // Invalid syntax
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
	db, runtime, mqttServer := setupTestRuntime(t)
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

	// Should timeout and not run forever
	if result.Success {
		t.Error("Expected execution to timeout")
	}

	if result.Error == nil || result.Error.Error() != "execution timeout after 200ms" {
		t.Errorf("Expected timeout error, got: %v", result.Error)
	}

	// Verify that execution was actually interrupted (should be close to 200ms, not forever)
	if duration > 300*time.Millisecond {
		t.Errorf("Execution took too long (%v), infinite loop not interrupted", duration)
	}

	t.Logf("✓ Infinite loop was interrupted after %v (expected ~200ms)", duration)
}
