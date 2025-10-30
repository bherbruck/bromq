package storage

import (
	"testing"
	"time"
)

func TestScriptStateCRUD(t *testing.T) {
	db := setupTestDB(t)

	scriptID := uint(123)

	// Test SetScriptState
	err := db.SetScriptState("script:123:counter", &scriptID, []byte("42"), nil)
	if err != nil {
		t.Fatalf("Failed to set state: %v", err)
	}

	// Test GetScriptState
	state, err := db.GetScriptState("script:123:counter")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if string(state.Value) != "42" {
		t.Errorf("Expected value '42', got '%s'", string(state.Value))
	}
	if state.ScriptID == nil || *state.ScriptID != scriptID {
		t.Errorf("Expected script ID %d, got %v", scriptID, state.ScriptID)
	}

	// Test Update (SetScriptState should upsert)
	err = db.SetScriptState("script:123:counter", &scriptID, []byte("100"), nil)
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	state, _ = db.GetScriptState("script:123:counter")
	if string(state.Value) != "100" {
		t.Errorf("Expected updated value '100', got '%s'", string(state.Value))
	}

	// Test DeleteScriptState
	err = db.DeleteScriptState("script:123:counter")
	if err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	_, err = db.GetScriptState("script:123:counter")
	if err == nil {
		t.Error("Expected error when getting deleted state")
	}
}

func TestScriptStateWithTTL(t *testing.T) {
	db := setupTestDB(t)

	scriptID := uint(123)
	expiresAt := time.Now().Add(100 * time.Millisecond)

	// Set state with expiration
	err := db.SetScriptState("script:123:temp", &scriptID, []byte("value"), &expiresAt)
	if err != nil {
		t.Fatalf("Failed to set state with TTL: %v", err)
	}

	// Should exist immediately
	state, err := db.GetScriptState("script:123:temp")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Expire states
	err = db.ExpireScriptStates()
	if err != nil {
		t.Fatalf("Failed to expire states: %v", err)
	}

	// Should no longer exist
	_, err = db.GetScriptState("script:123:temp")
	if err == nil {
		t.Error("Expected error when getting expired state")
	}
}

func TestScriptStateGlobalScope(t *testing.T) {
	db := setupTestDB(t)

	// Test global state (scriptID = nil)
	err := db.SetScriptState("global:shared_counter", nil, []byte("999"), nil)
	if err != nil {
		t.Fatalf("Failed to set global state: %v", err)
	}

	state, err := db.GetScriptState("global:shared_counter")
	if err != nil {
		t.Fatalf("Failed to get global state: %v", err)
	}
	if state.ScriptID != nil {
		t.Error("Expected global state to have nil ScriptID")
	}
	if string(state.Value) != "999" {
		t.Errorf("Expected value '999', got '%s'", string(state.Value))
	}
}

func TestBatchSetScriptState(t *testing.T) {
	db := setupTestDB(t)

	scriptID1 := uint(1)
	scriptID2 := uint(2)

	states := []ScriptState{
		{
			Key:      "script:1:key1",
			ScriptID: &scriptID1,
			Value:    []byte("value1"),
		},
		{
			Key:      "script:1:key2",
			ScriptID: &scriptID1,
			Value:    []byte("value2"),
		},
		{
			Key:      "script:2:key1",
			ScriptID: &scriptID2,
			Value:    []byte("value3"),
		},
		{
			Key:      "global:shared",
			ScriptID: nil,
			Value:    []byte("global_value"),
		},
	}

	err := db.BatchSetScriptState(states)
	if err != nil {
		t.Fatalf("Failed to batch set state: %v", err)
	}

	// Verify all states were set
	for _, s := range states {
		state, err := db.GetScriptState(s.Key)
		if err != nil {
			t.Errorf("Failed to get state for key %s: %v", s.Key, err)
		}
		if string(state.Value) != string(s.Value) {
			t.Errorf("Key %s: expected value '%s', got '%s'", s.Key, string(s.Value), string(state.Value))
		}
	}
}

func TestListScriptStateKeys(t *testing.T) {
	db := setupTestDB(t)

	scriptID1 := uint(1)
	scriptID2 := uint(2)

	// Set multiple states for different scripts and global
	states := []struct {
		key      string
		scriptID *uint
		value    string
	}{
		{"script:1:counter", &scriptID1, "1"},
		{"script:1:temperature", &scriptID1, "22.5"},
		{"script:1:humidity", &scriptID1, "65"},
		{"script:2:counter", &scriptID2, "2"},
		{"global:total", nil, "100"},
		{"global:average", nil, "50"},
	}

	for _, s := range states {
		db.SetScriptState(s.key, s.scriptID, []byte(s.value), nil)
	}

	// Test listing keys for script 1
	keys, err := db.ListScriptStateKeys(&scriptID1)
	if err != nil {
		t.Fatalf("Failed to list keys for script 1: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys for script 1, got %d: %v", len(keys), keys)
	}

	// Test listing keys for script 2
	keys, err = db.ListScriptStateKeys(&scriptID2)
	if err != nil {
		t.Fatalf("Failed to list keys for script 2: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("Expected 1 key for script 2, got %d", len(keys))
	}

	// Test listing global keys
	keys, err = db.ListScriptStateKeys(nil)
	if err != nil {
		t.Fatalf("Failed to list global keys: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Expected 2 global keys, got %d: %v", len(keys), keys)
	}
}

func TestScriptStateIsolation(t *testing.T) {
	db := setupTestDB(t)

	scriptID1 := uint(1)
	scriptID2 := uint(2)

	// Set same key name for different scripts
	db.SetScriptState("script:1:data", &scriptID1, []byte("script1_data"), nil)
	db.SetScriptState("script:2:data", &scriptID2, []byte("script2_data"), nil)

	// Verify isolation
	state1, _ := db.GetScriptState("script:1:data")
	state2, _ := db.GetScriptState("script:2:data")

	if string(state1.Value) != "script1_data" {
		t.Errorf("Script 1 state contaminated: %s", string(state1.Value))
	}
	if string(state2.Value) != "script2_data" {
		t.Errorf("Script 2 state contaminated: %s", string(state2.Value))
	}
}

func TestExpireScriptStates(t *testing.T) {
	db := setupTestDB(t)

	scriptID := uint(1)

	// Create states with different expirations
	now := time.Now()
	expired := now.Add(-1 * time.Hour)
	notExpired := now.Add(1 * time.Hour)

	states := []ScriptState{
		{
			Key:       "script:1:expired1",
			ScriptID:  &scriptID,
			Value:     []byte("old"),
			ExpiresAt: &expired,
		},
		{
			Key:       "script:1:expired2",
			ScriptID:  &scriptID,
			Value:     []byte("old"),
			ExpiresAt: &expired,
		},
		{
			Key:       "script:1:fresh",
			ScriptID:  &scriptID,
			Value:     []byte("new"),
			ExpiresAt: &notExpired,
		},
		{
			Key:       "script:1:no_expiry",
			ScriptID:  &scriptID,
			Value:     []byte("permanent"),
			ExpiresAt: nil,
		},
	}

	db.BatchSetScriptState(states)

	// Expire old states
	err := db.ExpireScriptStates()
	if err != nil {
		t.Fatalf("Failed to expire states: %v", err)
	}

	// Verify expired states are gone
	_, err = db.GetScriptState("script:1:expired1")
	if err == nil {
		t.Error("Expected expired1 to be deleted")
	}

	_, err = db.GetScriptState("script:1:expired2")
	if err == nil {
		t.Error("Expected expired2 to be deleted")
	}

	// Verify non-expired states still exist
	_, err = db.GetScriptState("script:1:fresh")
	if err != nil {
		t.Error("Expected fresh state to still exist")
	}

	_, err = db.GetScriptState("script:1:no_expiry")
	if err != nil {
		t.Error("Expected permanent state to still exist")
	}
}
