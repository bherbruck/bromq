package script

import (
	"github.com/prometheus/client_golang/prometheus"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	internalscript "github/bherbruck/bromq/internal/script"
	"github/bherbruck/bromq/internal/storage"
)

func setupTestHook(t *testing.T) (*storage.DB, *ScriptHook, *mqtt.Server) {
	t.Helper()

	// Setup in-memory database with shared cache mode
	// This ensures all connections see the same database (important for concurrent goroutines)
	config := storage.DefaultSQLiteConfig("file::memory:?cache=shared")
	cache := storage.NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := storage.OpenWithCache(config, cache)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Verify schema is ready (script tables exist)
	ensureScriptTablesExist(t, db)

	// Setup MQTT server
	mqttServer := mqtt.New(&mqtt.Options{
		InlineClient: true,
	})
	if err := mqttServer.Serve(); err != nil {
		t.Fatalf("failed to start MQTT server: %v", err)
	}

	// Setup script engine and hook
	engine := internalscript.NewEngine(db, mqttServer)
	engine.Start()

	hook := NewScriptHook(engine)

	return db, hook, mqttServer
}

// ensureScriptTablesExist verifies that script-related tables exist in the database
func ensureScriptTablesExist(t *testing.T, db *storage.DB) {
	t.Helper()

	// Verify all script tables exist
	requiredTables := []string{"scripts", "script_triggers", "script_logs", "script_state"}
	for _, tableName := range requiredTables {
		var exists int64
		err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&exists).Error
		if err != nil || exists == 0 {
			t.Fatalf("required table %s does not exist (err: %v)", tableName, err)
		}
	}
}

func TestScriptHookID(t *testing.T) {
	_, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	if hook.ID() != "script-hook" {
		t.Errorf("Expected hook ID 'script-hook', got '%s'", hook.ID())
	}
}

func TestScriptHookProvides(t *testing.T) {
	_, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	tests := []struct {
		name     string
		hookType byte
		expected bool
	}{
		{"OnPublish", mqtt.OnPublish, true},
		{"OnConnect", mqtt.OnConnect, true},
		{"OnDisconnect", mqtt.OnDisconnect, true},
		{"OnSubscribe", mqtt.OnSubscribe, true},
		{"OnSubscribed", mqtt.OnSubscribed, true},
		{"OnUnsubscribe", mqtt.OnUnsubscribe, false},
		{"OnSessionEstablished", mqtt.OnSessionEstablished, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hook.Provides(tt.hookType)
			if result != tt.expected {
				t.Errorf("Provides(%d): expected %v, got %v", tt.hookType, tt.expected, result)
			}
		})
	}
}

func TestScriptHookOnPublish(t *testing.T) {
	db, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	// Create script that logs publish events
	script, _ := db.CreateScript("log-publish", "", `
		log.info("Published to " + msg.topic + ": " + msg.payload);
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "test/#", Priority: 100, Enabled: true},
	})

	// Create mock client
	cl := &mqtt.Client{
		ID: "test-client",
		Properties: mqtt.ClientProperties{
			Username: []byte("test-user"),
		},
	}

	// Create publish packet
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Qos:    1,
			Retain: false,
		},
		TopicName: "test/topic",
		Payload:   []byte("hello world"),
	}

	// Call OnPublish
	returnedPk, err := hook.OnPublish(cl, pk)
	if err != nil {
		t.Errorf("OnPublish returned error: %v", err)
	}

	// Verify packet is returned unchanged
	if returnedPk.TopicName != pk.TopicName {
		t.Error("Packet was modified")
	}

	// Give script time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify script executed
	_, total, _ := db.ListScriptLogs(script.ID, 1, 10, "")
	if total == 0 {
		t.Error("Expected script to have executed")
	}
}

func TestScriptHookOnConnect(t *testing.T) {
	db, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	// Create script that logs connect events
	script, _ := db.CreateScript("log-connect", "", `
		log.info("Client connected: " + msg.clientId + " (" + msg.username + ")");
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_connect", Topic: "", Priority: 100, Enabled: true},
	})

	// Create mock client
	cl := &mqtt.Client{
		ID: "client-123",
		Properties: mqtt.ClientProperties{
			Username: []byte("user-456"),
		},
	}

	// Create connect packet
	pk := packets.Packet{
		Connect: packets.ConnectParams{
			Clean: true,
		},
	}

	// Call OnConnect
	err := hook.OnConnect(cl, pk)
	if err != nil {
		t.Errorf("OnConnect returned error: %v", err)
	}

	// Give script time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify script executed
	logs, total, _ := db.ListScriptLogs(script.ID, 1, 10, "")
	if total == 0 {
		t.Error("Expected script to have executed")
	}

	if len(logs) > 0 && logs[0].Message != "Client connected: client-123 (user-456)" {
		t.Errorf("Unexpected log message: %s", logs[0].Message)
	}
}

func TestScriptHookOnDisconnect(t *testing.T) {
	db, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	// Create script that logs disconnect events
	script, _ := db.CreateScript("log-disconnect", "", `
		var message = "Client disconnected: " + msg.clientId;
		if (msg.error) {
			message += " (error: " + msg.error + ")";
		}
		log.info(message);
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_disconnect", Topic: "", Priority: 100, Enabled: true},
	})

	// Create mock client
	cl := &mqtt.Client{
		ID: "client-123",
		Properties: mqtt.ClientProperties{
			Username: []byte("user-456"),
		},
	}

	// Call OnDisconnect with no error
	hook.OnDisconnect(cl, nil, false)

	// Give script time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify script executed
	logs, total, _ := db.ListScriptLogs(script.ID, 1, 10, "")
	if total == 0 {
		t.Error("Expected script to have executed")
	}

	if len(logs) > 0 && logs[0].Message != "Client disconnected: client-123" {
		t.Errorf("Unexpected log message: %s", logs[0].Message)
	}
}

func TestScriptHookOnSubscribe(t *testing.T) {
	db, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	// Create script that sets state on subscribe (more reliable for async test)
	script, err := db.CreateScript("log-subscribe", "", `
		state.set("subscribed_topic", msg.topic);
	`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_subscribe", Topic: "test/#", Priority: 100, Enabled: true},
	})
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	// Create mock client
	cl := &mqtt.Client{
		ID: "client-123",
		Properties: mqtt.ClientProperties{
			Username: []byte("user-456"),
		},
	}

	// Create subscribe packet
	pk := packets.Packet{
		Filters: packets.Subscriptions{
			{Filter: "test/topic", Qos: 1},
			{Filter: "other/topic", Qos: 0},
		},
	}

	// Call OnSubscribe
	returnedPk := hook.OnSubscribe(cl, pk)

	// Verify packet is returned unchanged
	if len(returnedPk.Filters) != len(pk.Filters) {
		t.Error("Packet was modified")
	}

	// Give script time to execute (subscribe events need slightly more time)
	time.Sleep(350 * time.Millisecond)

	// Verify script executed by checking state (more reliable for async tests)
	topic, exists := hook.engine.GetState().Get(&script.ID, "subscribed_topic")
	if !exists {
		t.Error("Expected script to have executed")
	}
	if exists && topic != "test/topic" {
		t.Errorf("Expected subscribed to 'test/topic', got: %v", topic)
	}
}

func TestScriptHookMultipleScripts(t *testing.T) {
	db, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	// Create multiple scripts that set state
	script1, _ := db.CreateScript("script-1", "", `state.set("ran", true);`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "test/#", Priority: 100, Enabled: true},
	})

	script2, _ := db.CreateScript("script-2", "", `state.set("ran", true);`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "test/#", Priority: 50, Enabled: true},
	})

	script3, _ := db.CreateScript("script-3", "", `state.set("ran", true);`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "test/#", Priority: 150, Enabled: true},
	})

	// Create mock client and packet
	cl := &mqtt.Client{
		ID: "test-client",
		Properties: mqtt.ClientProperties{
			Username: []byte("test-user"),
		},
	}

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "test/topic",
		Payload:   []byte("test"),
	}

	// Call OnPublish
	hook.OnPublish(cl, pk)

	// Give scripts time to execute
	time.Sleep(100 * time.Millisecond)

	// Verify all scripts executed by checking state
	state := hook.engine.GetState()

	_, exists1 := state.Get(&script1.ID, "ran")
	_, exists2 := state.Get(&script2.ID, "ran")
	_, exists3 := state.Get(&script3.ID, "ran")

	if !exists1 {
		t.Error("Expected script 1 to execute")
	}
	if !exists2 {
		t.Error("Expected script 2 to execute")
	}
	if !exists3 {
		t.Error("Expected script 3 to execute")
	}
}

func TestScriptHookTopicing(t *testing.T) {
	db, hook, mqttServer := setupTestHook(t)
	defer mqttServer.Close()

	// Create scripts with different topic filters that set state
	scriptWildcard, _ := db.CreateScript("wildcard", "", `state.set("matched", true);`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "sensors/#", Priority: 100, Enabled: true},
	})

	scriptSpecific, _ := db.CreateScript("specific", "", `state.set("matched", true);`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "sensors/+/temp", Priority: 100, Enabled: true},
	})

	scriptNoMatch, _ := db.CreateScript("no-match", "", `state.set("matched", true);`, true, []byte("{}"), []storage.ScriptTrigger{
		{Type: "on_publish", Topic: "other/#", Priority: 100, Enabled: true},
	})

	// Create mock client
	cl := &mqtt.Client{
		ID: "test-client",
		Properties: mqtt.ClientProperties{
			Username: []byte("test-user"),
		},
	}

	// Publish to sensors/room1/temp
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish},
		TopicName:   "sensors/room1/temp",
		Payload:     []byte("22.5"),
	}

	hook.OnPublish(cl, pk)
	time.Sleep(100 * time.Millisecond)

	// Verify matching scripts executed by checking state
	state := hook.engine.GetState()

	_, exists1 := state.Get(&scriptWildcard.ID, "matched")
	_, exists2 := state.Get(&scriptSpecific.ID, "matched")
	_, exists3 := state.Get(&scriptNoMatch.ID, "matched")

	if !exists1 {
		t.Error("Expected wildcard script to execute")
	}
	if !exists2 {
		t.Error("Expected specific script to execute")
	}
	if exists3 {
		t.Error("Expected no-match script NOT to execute")
	}
}
