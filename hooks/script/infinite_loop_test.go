package script

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	mqtt "github.com/mochi-mqtt/server/v2"

	"github/bherbruck/bromq/internal/badgerstore"
	internalscript "github/bherbruck/bromq/internal/script"
	"github/bherbruck/bromq/internal/storage"
)

func setupTestEngine(t *testing.T) (*storage.DB, *internalscript.Engine, *mqtt.Server) {
	t.Helper()

	// Setup in-memory database with shared cache mode
	// This ensures all connections see the same database (important for concurrent goroutines)
	config := storage.DefaultSQLiteConfig("file::memory:?cache=shared")
	cache := storage.NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := storage.OpenWithCache(config, cache)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Verify schema is ready
	ensureScriptTablesExist(t, db)

	// Setup MQTT server
	mqttServer := mqtt.New(&mqtt.Options{
		InlineClient: true,
	})
	if err := mqttServer.Serve(); err != nil {
		t.Fatalf("failed to start MQTT server: %v", err)
	}

	// Setup BadgerDB for state
	badger := badgerstore.OpenInMemory(t)

	// Setup script engine
	engine := internalscript.NewEngine(db, badger, mqttServer)
	engine.Start()

	// Add script hook (critical for script-to-script communication)
	hook := NewScriptHook(engine)
	mqttServer.AddHook(hook, nil)

	return db, engine, mqttServer
}

func TestPreventSelfTriggering(t *testing.T) {
	db, engine, mqttServer := setupTestEngine(t)
	defer mqttServer.Close()
	defer engine.Shutdown(context.Background())

	// Create a script that would cause infinite loop by republishing to the same topic
	script, _ := db.CreateScript("infinite-loop-script", "", `
		// This would cause infinite loop without prevention
		state.set("execution_count", (state.get("execution_count") || 0) + 1);
		mqtt.publish(msg.topic, msg.payload, 0, false);
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "test/#", Priority: 100, Enabled: true},
	})

	// Reload cache after creating scripts
	engine.ReloadScripts()

	message := &internalscript.Message{
		Type:     "publish",
		Topic:    "test/loop",
		Payload:  "trigger",
		ClientID: "external-client",
	}

	// Execute the script
	engine.ExecuteForTrigger("on_publish", "test/loop", message)

	// Give enough time for potential loop iterations
	time.Sleep(500 * time.Millisecond)

	// Check execution count - should be exactly 1 (not infinite)
	count, exists := engine.GetState().Get(&script.ID, "execution_count")
	if !exists {
		t.Fatal("Script did not execute at all")
	}

	// Handle both int64 and float64 (depending on how goja serializes)
	var executionCount int
	switch v := count.(type) {
	case float64:
		executionCount = int(v)
	case int64:
		executionCount = int(v)
	case int:
		executionCount = v
	default:
		t.Fatalf("Unexpected type for execution_count: %T", count)
	}

	if executionCount != 1 {
		t.Errorf("Expected script to execute exactly once, got %d executions (infinite loop not prevented!)", executionCount)
	}
}

func TestAllowScriptChaining(t *testing.T) {
	db, engine, mqttServer := setupTestEngine(t)
	defer mqttServer.Close()
	defer engine.Shutdown(context.Background())

	// Script A: publishes to topic B
	_, _ = db.CreateScript("script-a", "", `
		global.set("script_a_ran", true);
		mqtt.publish("topic/b", "from_a", 0, false);
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "topic/a", Priority: 100, Enabled: true},
	})

	// Script B: listens to topic B (should trigger)
	_, _ = db.CreateScript("script-b", "", `
		global.set("script_b_ran", true);
		global.set("script_b_payload", msg.payload);
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "topic/b", Priority: 100, Enabled: true},
	})

	// Reload cache after creating scripts
	engine.ReloadScripts()

	// Trigger Script A
	message := &internalscript.Message{
		Type:     "publish",
		Topic:    "topic/a",
		Payload:  "trigger",
		ClientID: "external-client",
	}

	engine.ExecuteForTrigger("on_publish", "topic/a", message)

	// Give time for both scripts to execute (Script A runs, publishes, then Script B should run)
	time.Sleep(1000 * time.Millisecond)

	// Verify Script A ran
	_, aRan := engine.GetState().Get(nil, "script_a_ran")
	if !aRan {
		t.Error("Script A did not execute")
	}

	// Verify Script B ran (script chaining works)
	_, bRan := engine.GetState().Get(nil, "script_b_ran")
	if !bRan {
		t.Error("Script B did not execute (script chaining broken!)")
	}

	// Verify Script B received the correct payload
	payload, _ := engine.GetState().Get(nil, "script_b_payload")
	if payload != "from_a" {
		t.Errorf("Script B got wrong payload: %v", payload)
	}

	t.Log("✓ Script chaining works: Script A → Script B")
}
