package script

import (
	"context"
	"testing"
	"time"

	"github/bherbruck/bromq/internal/storage"
)

func TestScriptAPIMqttPublish(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "mqtt-publish-test",
		ScriptContent: `
			mqtt.publish("output/topic", "hello world", 1, false);
			log.info("Published message");
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "input/topic",
		Payload:  "trigger",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	// Verify message was published to MQTT server
	// Note: In a real integration test, we'd subscribe and verify the message
	// For now, we just verify the script executed without error
}

func TestScriptAPIMqttPublishInvalidQoS(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "invalid-qos",
		ScriptContent: `
			mqtt.publish("output/topic", "hello", 3, false); // Invalid QoS
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "input/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	if result.Success {
		t.Error("Expected execution to fail with invalid QoS")
	}
}

func TestScriptAPIStateSetGet(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "state-test",
		ScriptContent: `
			// Set values
			state.set("counter", 42);
			state.set("name", "test");
			state.set("object", {a: 1, b: 2});

			// Get values
			var counter = state.get("counter");
			var name = state.get("name");
			var obj = state.get("object");

			if (counter !== 42) throw new Error("Counter mismatch");
			if (name !== "test") throw new Error("Name mismatch");
			if (obj.a !== 1) throw new Error("Object mismatch");

			log.info("State operations successful");
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
}

func TestScriptAPIStateWithTTL(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "state-ttl-test",
		ScriptContent: `
			// Set value with 1 second TTL
			state.set("temp", "value", {ttl: 1});

			// Should exist immediately
			var val = state.get("temp");
			if (val !== "value") throw new Error("Value not found");

			log.info("TTL state set successfully");
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

	// Wait for TTL to expire
	time.Sleep(1100 * time.Millisecond)

	// Run script again to check if value expired
	script2 := &storage.Script{
		ID:   1,
		Name: "state-ttl-check",
		ScriptContent: `
			var val = state.get("temp");
			if (val !== undefined) {
				throw new Error("Value should have expired");
			}
			log.info("Value correctly expired");
		`,
	}

	result2 := runtime.Execute(ctx, script2, message)
	if !result2.Success {
		t.Errorf("Expected success, got error: %v", result2.Error)
	}
}

func TestScriptAPIStateDelete(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "state-delete-test",
		ScriptContent: `
			state.set("toDelete", "value");

			var val1 = state.get("toDelete");
			if (val1 !== "value") throw new Error("Value not set");

			state.delete("toDelete");

			var val2 = state.get("toDelete");
			if (val2 !== undefined) throw new Error("Value not deleted");

			log.info("Delete successful");
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
}

func TestScriptAPIStateKeys(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "state-keys-test",
		ScriptContent: `
			state.set("key1", "value1");
			state.set("key2", "value2");
			state.set("key3", "value3");

			var keys = state.keys();
			if (keys.length !== 3) throw new Error("Expected 3 keys, got " + keys.length);

			log.info("Keys: " + keys.join(", "));
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
}

func TestScriptAPIGlobalState(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Script 1 sets global state
	script1 := &storage.Script{
		ID:   1,
		Name: "global-set",
		ScriptContent: `
			global.set("shared_counter", 100);
			log.info("Set global counter");
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result1 := runtime.Execute(ctx, script1, message)
	if !result1.Success {
		t.Errorf("Script 1 failed: %v", result1.Error)
	}

	// Script 2 (different script ID) reads global state
	script2 := &storage.Script{
		ID:   2,
		Name: "global-get",
		ScriptContent: `
			var counter = global.get("shared_counter");
			if (counter !== 100) throw new Error("Global state not shared: " + counter);
			log.info("Read global counter: " + counter);
		`,
	}

	result2 := runtime.Execute(ctx, script2, message)
	if !result2.Success {
		t.Errorf("Script 2 failed: %v", result2.Error)
	}
}

func TestScriptAPIStateIsolation(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Script 1 sets script-scoped state
	script1 := &storage.Script{
		ID:   1,
		Name: "script1",
		ScriptContent: `
			state.set("data", "script1_data");
			log.info("Script 1 set data");
		`,
	}

	message := &Message{
		Type:     "publish",
		Topic:    "test/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result1 := runtime.Execute(ctx, script1, message)
	if !result1.Success {
		t.Errorf("Script 1 failed: %v", result1.Error)
	}

	// Script 2 should NOT see script 1's state
	script2 := &storage.Script{
		ID:   2,
		Name: "script2",
		ScriptContent: `
			var data = state.get("data");
			if (data !== undefined) {
				throw new Error("Script 2 should not see script 1's state");
			}
			log.info("Script 2 correctly isolated");
		`,
	}

	result2 := runtime.Execute(ctx, script2, message)
	if !result2.Success {
		t.Errorf("Script 2 failed: %v", result2.Error)
	}
}

func TestScriptAPIComplexDataTypes(t *testing.T) {
	_, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	script := &storage.Script{
		ID:   1,
		Name: "complex-types",
		ScriptContent: `
			// Test arrays
			state.set("array", [1, 2, 3, "four", {five: 5}]);
			var arr = state.get("array");
			if (arr.length !== 5) throw new Error("Array length mismatch");
			if (arr[3] !== "four") throw new Error("Array value mismatch");

			// Test nested objects
			state.set("nested", {
				level1: {
					level2: {
						value: "deep"
					}
				}
			});
			var nested = state.get("nested");
			if (nested.level1.level2.value !== "deep") throw new Error("Nested object mismatch");

			log.info("Complex types work correctly");
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
}

func TestScriptAPIMqttPublishRateLimit(t *testing.T) {
	db, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Set a low rate limit for testing
	runtime.SetMaxPublishes(5)

	// Script that tries to publish more than the limit
	script, err := db.CreateScript("rate-limit-test", "", `
		for (var i = 0; i < 10; i++) {
			mqtt.publish("test/topic/" + i, "message " + i, 0, false);
		}
	`, true, []byte("{}"), []storage.ScriptTrigger{})
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	message := &Message{
		Type:     "publish",
		Topic:    "trigger/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	// Should fail with rate limit error
	if result.Success {
		t.Error("Expected execution to fail due to rate limit")
	}

	if result.Error == nil {
		t.Error("Expected rate limit error")
	}

	// Verify error message mentions rate limit
	errorMsg := result.Error.Error()
	if !contains(errorMsg, "rate limit") {
		t.Errorf("Expected rate limit error, got: %s", errorMsg)
	}

	t.Logf("✓ Rate limit enforced: %v", result.Error)
}

func TestScriptAPIMqttPublishWithinLimit(t *testing.T) {
	db, runtime, mqttServer := setupTestRuntime(t)
	defer mqttServer.Close()

	// Set rate limit
	runtime.SetMaxPublishes(5)

	// Script that publishes exactly at the limit
	script, err := db.CreateScript("within-limit-test", "", `
		for (var i = 0; i < 5; i++) {
			mqtt.publish("test/topic/" + i, "message " + i, 0, false);
		}
		log.info("Published 5 messages");
	`, true, []byte("{}"), []storage.ScriptTrigger{})
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	message := &Message{
		Type:     "publish",
		Topic:    "trigger/topic",
		Payload:  "test",
		ClientID: "test-client",
	}

	ctx := context.Background()
	result := runtime.Execute(ctx, script, message)

	// Should succeed (exactly at limit)
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	t.Log("✓ Script successfully published 5 messages (at limit)")
}

// Helper function for string contains check
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
