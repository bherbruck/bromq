package script

// StateStore defines the interface for script state storage
// Implemented by both StateManager (SQLite) and StateManagerBadger (BadgerDB)
type StateStore interface {
	// Start initializes background workers (if any)
	Start()

	// Stop shuts down background workers and flushes state
	Stop() error

	// Set stores a value in state (script-scoped or global)
	Set(scriptID *uint, key string, value interface{}, ttl *int) error

	// Get retrieves a value from state
	Get(scriptID *uint, key string) (interface{}, bool)

	// Delete removes a value from state
	Delete(scriptID *uint, key string) error

	// Keys returns all keys for a script or global state
	Keys(scriptID *uint) []string

	// FlushDirty flushes dirty cache entries to storage (no-op for BadgerDB)
	FlushDirty() error

	// FlushAll flushes all cache entries to storage (no-op for BadgerDB)
	FlushAll() error
}

// DeleteAllForScript is an optional method for cleanup
type StateStoreWithCleanup interface {
	StateStore
	DeleteAllForScript(scriptID uint) error
}
