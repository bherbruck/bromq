package script

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github/bherbruck/bromq/internal/storage"
)

// StateValue represents a cached state value
type StateValue struct {
	Value     interface{}
	ExpiresAt *time.Time
	ScriptID  *uint
	Dirty     bool // Tracks if value has been modified since last flush
}

// StateManager manages script state with in-memory caching and periodic DB persistence
type StateManager struct {
	db          *storage.DB
	cache       sync.Map // key -> *StateValue
	flushTicker *time.Ticker
	expireTicker *time.Ticker
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewStateManager creates a new state manager
func NewStateManager(db *storage.DB) *StateManager {
	return &StateManager{
		db:       db,
		stopChan: make(chan struct{}),
	}
}

// Start starts the background flush and expiration workers
func (s *StateManager) Start() {
	s.wg.Add(2)
	go s.flushWorker()
	go s.expirationWorker()
	slog.Info("State manager started")
}

// Stop stops the background workers and performs final flush
func (s *StateManager) Stop() error {
	slog.Info("Stopping state manager...")
	close(s.stopChan)
	s.wg.Wait()

	// Final flush
	if err := s.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush state on shutdown: %w", err)
	}

	slog.Info("State manager stopped")
	return nil
}

// flushWorker periodically flushes dirty state to database
func (s *StateManager) flushWorker() {
	defer s.wg.Done()

	s.flushTicker = time.NewTicker(5 * time.Second)
	defer s.flushTicker.Stop()

	for {
		select {
		case <-s.flushTicker.C:
			if err := s.FlushDirty(); err != nil {
				slog.Error("Failed to flush state", "error", err)
			}
		case <-s.stopChan:
			slog.Debug("Flush worker stopping")
			return
		}
	}
}

// expirationWorker periodically removes expired state entries and publish tracking
func (s *StateManager) expirationWorker() {
	defer s.wg.Done()

	s.expireTicker = time.NewTicker(5 * time.Minute)
	defer s.expireTicker.Stop()

	// Also cleanup publish tracker more frequently (every 10 seconds)
	publishCleanupTicker := time.NewTicker(10 * time.Second)
	defer publishCleanupTicker.Stop()

	for {
		select {
		case <-s.expireTicker.C:
			s.cleanupExpired()
			// Also cleanup expired entries in database
			if err := s.db.ExpireScriptStates(); err != nil {
				slog.Error("Failed to expire states in database", "error", err)
			}
		case <-publishCleanupTicker.C:
			// Cleanup expired publish tracking entries
			scriptPublishTracker.cleanup()
		case <-s.stopChan:
			slog.Debug("Expiration worker stopping")
			return
		}
	}
}

// cleanupExpired removes expired entries from cache
func (s *StateManager) cleanupExpired() {
	now := time.Now()
	expiredKeys := make([]string, 0)

	s.cache.Range(func(key, value interface{}) bool {
		stateVal := value.(*StateValue)
		if stateVal.ExpiresAt != nil && stateVal.ExpiresAt.Before(now) {
			expiredKeys = append(expiredKeys, key.(string))
		}
		return true
	})

	for _, key := range expiredKeys {
		s.cache.Delete(key)
	}

	if len(expiredKeys) > 0 {
		slog.Debug("Cleaned up expired state entries", "count", len(expiredKeys))
	}
}

// Set stores a value in state (script-scoped or global)
func (s *StateManager) Set(scriptID *uint, key string, value interface{}, ttl *int) error {
	// Build full key
	fullKey := s.buildKey(scriptID, key)

	// Calculate expiration
	var expiresAt *time.Time
	if ttl != nil && *ttl > 0 {
		exp := time.Now().Add(time.Duration(*ttl) * time.Second)
		expiresAt = &exp
	}

	// Store in cache
	stateVal := &StateValue{
		Value:     value,
		ExpiresAt: expiresAt,
		ScriptID:  scriptID,
		Dirty:     true,
	}
	s.cache.Store(fullKey, stateVal)

	return nil
}

// Get retrieves a value from state
func (s *StateManager) Get(scriptID *uint, key string) (interface{}, bool) {
	fullKey := s.buildKey(scriptID, key)

	// Try cache first
	if val, ok := s.cache.Load(fullKey); ok {
		stateVal := val.(*StateValue)

		// Check expiration
		if stateVal.ExpiresAt != nil && stateVal.ExpiresAt.Before(time.Now()) {
			s.cache.Delete(fullKey)
			return nil, false
		}

		return stateVal.Value, true
	}

	// Load from database if not in cache
	dbState, err := s.db.GetScriptState(fullKey)
	if err != nil {
		return nil, false
	}

	// Check database expiration
	if dbState.ExpiresAt != nil && dbState.ExpiresAt.Before(time.Now()) {
		return nil, false
	}

	// Deserialize value
	var value interface{}
	if err := json.Unmarshal(dbState.Value, &value); err != nil {
		slog.Error("Failed to deserialize state value", "key", fullKey, "error", err)
		return nil, false
	}

	// Cache it
	stateVal := &StateValue{
		Value:     value,
		ExpiresAt: dbState.ExpiresAt,
		ScriptID:  scriptID,
		Dirty:     false,
	}
	s.cache.Store(fullKey, stateVal)

	return value, true
}

// Delete removes a value from state
func (s *StateManager) Delete(scriptID *uint, key string) error {
	fullKey := s.buildKey(scriptID, key)

	// Remove from cache
	s.cache.Delete(fullKey)

	// Remove from database
	return s.db.DeleteScriptState(fullKey)
}

// Keys returns all keys for a script (or global)
func (s *StateManager) Keys(scriptID *uint) []string {
	prefix := s.buildPrefix(scriptID)
	keys := make([]string, 0)

	// Collect from cache
	s.cache.Range(func(key, value interface{}) bool {
		k := key.(string)
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			// Strip prefix to get user key
			userKey := k[len(prefix):]
			keys = append(keys, userKey)
		}
		return true
	})

	// Also check database for keys not in cache
	dbKeys, err := s.db.ListScriptStateKeys(scriptID)
	if err != nil {
		slog.Error("Failed to list state keys from database", "error", err)
		return keys
	}

	// Merge (avoiding duplicates)
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, k := range dbKeys {
		// Strip prefix
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			userKey := k[len(prefix):]
			if !keySet[userKey] {
				keys = append(keys, userKey)
				keySet[userKey] = true
			}
		}
	}

	return keys
}

// FlushDirty flushes only modified (dirty) state values to database
func (s *StateManager) FlushDirty() error {
	dirtyStates := make([]storage.ScriptState, 0)

	s.cache.Range(func(key, value interface{}) bool {
		stateVal := value.(*StateValue)
		if stateVal.Dirty {
			// Serialize value
			jsonValue, err := json.Marshal(stateVal.Value)
			if err != nil {
				slog.Error("Failed to serialize state value", "key", key, "error", err)
				return true // Continue iteration
			}

			dirtyStates = append(dirtyStates, storage.ScriptState{
				Key:       key.(string),
				ScriptID:  stateVal.ScriptID,
				Value:     jsonValue,
				ExpiresAt: stateVal.ExpiresAt,
			})

			// Mark as clean
			stateVal.Dirty = false
			s.cache.Store(key, stateVal)
		}
		return true
	})

	if len(dirtyStates) == 0 {
		return nil
	}

	slog.Debug("Flushing dirty state", "count", len(dirtyStates))

	// Batch write to database
	return s.db.BatchSetScriptState(dirtyStates)
}

// FlushAll flushes all state values to database (used on shutdown)
func (s *StateManager) FlushAll() error {
	allStates := make([]storage.ScriptState, 0)

	s.cache.Range(func(key, value interface{}) bool {
		stateVal := value.(*StateValue)

		// Serialize value
		jsonValue, err := json.Marshal(stateVal.Value)
		if err != nil {
			slog.Error("Failed to serialize state value", "key", key, "error", err)
			return true // Continue iteration
		}

		allStates = append(allStates, storage.ScriptState{
			Key:       key.(string),
			ScriptID:  stateVal.ScriptID,
			Value:     jsonValue,
			ExpiresAt: stateVal.ExpiresAt,
		})

		return true
	})

	if len(allStates) == 0 {
		slog.Info("No state to flush")
		return nil
	}

	slog.Info("Flushing all state to database", "count", len(allStates))

	// Batch write to database
	return s.db.BatchSetScriptState(allStates)
}

// buildKey constructs the full storage key for a value
func (s *StateManager) buildKey(scriptID *uint, userKey string) string {
	if scriptID == nil {
		return fmt.Sprintf("global:%s", userKey)
	}
	return fmt.Sprintf("script:%d:%s", *scriptID, userKey)
}

// buildPrefix constructs the prefix for filtering keys
func (s *StateManager) buildPrefix(scriptID *uint) string {
	if scriptID == nil {
		return "global:"
	}
	return fmt.Sprintf("script:%d:", *scriptID)
}
