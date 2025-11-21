package script

import (
	"fmt"
	"time"

	"github/bherbruck/bromq/internal/badgerstore"
)

// StateManagerBadger manages script state using BadgerDB (no caching needed)
type StateManagerBadger struct {
	badger *badgerstore.BadgerStore
}

// NewStateManagerBadger creates a new state manager using BadgerDB
func NewStateManagerBadger(badger *badgerstore.BadgerStore) *StateManagerBadger {
	return &StateManagerBadger{
		badger: badger,
	}
}

// Start is a no-op for BadgerDB (no background workers needed)
func (s *StateManagerBadger) Start() {
	// BadgerDB handles its own GC internally
	// No flush workers needed - writes are direct
	// No expiration workers needed - TTL is native
}

// Stop is a no-op for BadgerDB
func (s *StateManagerBadger) Stop() error {
	// No state to flush - all writes are immediate
	return nil
}

// Set stores a value in state (script-scoped or global)
func (s *StateManagerBadger) Set(scriptID *uint, key string, value interface{}, ttl *int) error {
	fullKey := buildKey(scriptID, key)

	// Calculate TTL duration
	var ttlDuration time.Duration
	if ttl != nil && *ttl > 0 {
		ttlDuration = time.Duration(*ttl) * time.Second
	}

	// Write directly to BadgerDB (no caching)
	return s.badger.SetScriptState(fullKey, scriptID, value, ttlDuration)
}

// Get retrieves a value from state
func (s *StateManagerBadger) Get(scriptID *uint, key string) (interface{}, bool) {
	fullKey := buildKey(scriptID, key)

	state, err := s.badger.GetScriptState(fullKey)
	if err != nil || state == nil {
		return nil, false
	}

	return state.Value, true
}

// Delete removes a value from state
func (s *StateManagerBadger) Delete(scriptID *uint, key string) error {
	fullKey := buildKey(scriptID, key)
	return s.badger.DeleteScriptState(fullKey)
}

// Keys returns all keys for a script or global state
func (s *StateManagerBadger) Keys(scriptID *uint) []string {
	keys, err := s.badger.ListScriptStateKeys(scriptID)
	if err != nil {
		return []string{}
	}

	// Strip the prefix from keys to return user-facing keys
	var userKeys []string
	prefix := buildPrefix(scriptID)
	for _, key := range keys {
		if len(key) > len(prefix) {
			userKeys = append(userKeys, key[len(prefix):])
		}
	}

	return userKeys
}

// DeleteAllForScript deletes all state for a specific script
func (s *StateManagerBadger) DeleteAllForScript(scriptID uint) error {
	return s.badger.DeleteScriptStates(scriptID)
}

// FlushDirty is a no-op for BadgerDB (writes are immediate)
func (s *StateManagerBadger) FlushDirty() error {
	return nil
}

// FlushAll is a no-op for BadgerDB (writes are immediate)
func (s *StateManagerBadger) FlushAll() error {
	return nil
}

// buildKey constructs the full storage key for a value
func buildKey(scriptID *uint, userKey string) string {
	if scriptID == nil {
		return fmt.Sprintf("global:%s", userKey)
	}
	return fmt.Sprintf("script:%d:%s", *scriptID, userKey)
}

// buildPrefix constructs the prefix for listing keys
func buildPrefix(scriptID *uint) string {
	if scriptID == nil {
		return "global:"
	}
	return fmt.Sprintf("script:%d:", *scriptID)
}
