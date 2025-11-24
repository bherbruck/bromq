package badgerstore

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// ScriptStateValue represents a script state entry in BadgerDB
type ScriptStateValue struct {
	Value     interface{} `json:"value"`
	ScriptID  *uint       `json:"script_id,omitempty"`
	ExpiresAt *time.Time  `json:"expires_at,omitempty"`
}

// GetScriptState retrieves a state value by key
func (b *BadgerStore) GetScriptState(key string) (*ScriptStateValue, error) {
	data, err := b.Get(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil // Not found
	}

	var state ScriptStateValue
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Check if expired
	if state.ExpiresAt != nil && state.ExpiresAt.Before(time.Now()) {
		// Delete expired entry
		_ = b.Delete(key)
		return nil, nil
	}

	return &state, nil
}

// SetScriptState creates or updates a state value
func (b *BadgerStore) SetScriptState(key string, scriptID *uint, value interface{}, ttl time.Duration) error {
	state := ScriptStateValue{
		Value:    value,
		ScriptID: scriptID,
	}

	if ttl > 0 {
		expiresAt := time.Now().Add(ttl)
		state.ExpiresAt = &expiresAt
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return b.Set(key, data, ttl)
}

// DeleteScriptState deletes a state value by key
func (b *BadgerStore) DeleteScriptState(key string) error {
	return b.Delete(key)
}

// ListScriptStateKeys returns all keys for a specific script or global state
func (b *BadgerStore) ListScriptStateKeys(scriptID *uint) ([]string, error) {
	var prefix string
	if scriptID == nil {
		prefix = "global:"
	} else {
		prefix = fmt.Sprintf("script:%d:", *scriptID)
	}

	return b.ListKeysWithPrefix(prefix)
}

// DeleteScriptStates deletes all state values for a specific script
func (b *BadgerStore) DeleteScriptStates(scriptID uint) error {
	prefix := fmt.Sprintf("script:%d:", scriptID)
	return b.DeletePrefix(prefix)
}

// BatchSetScriptState sets multiple state values in a single transaction
func (b *BadgerStore) BatchSetScriptState(states map[string]*ScriptStateValue) error {
	batch := make(map[string][]byte)

	for key, state := range states {
		data, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("failed to marshal state %s: %w", key, err)
		}
		batch[key] = data
	}

	// Determine TTL from first state with expiration (if any)
	// Note: All states in a batch should ideally have similar TTLs
	var ttl time.Duration
	for _, state := range states {
		if state.ExpiresAt != nil {
			ttl = time.Until(*state.ExpiresAt)
			break
		}
	}

	return b.BatchSet(batch, ttl)
}

// CountScriptStates returns the number of state entries for a script
func (b *BadgerStore) CountScriptStates(scriptID *uint) (int64, error) {
	keys, err := b.ListScriptStateKeys(scriptID)
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// GetAllScriptStates returns all state entries (for migration/debugging)
func (b *BadgerStore) GetAllScriptStates() (map[string]*ScriptStateValue, error) {
	states := make(map[string]*ScriptStateValue)

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.KeyCopy(nil))

			// Skip non-state keys (if we store other data in BadgerDB)
			if key[:7] != "script:" && key[:7] != "global:" {
				continue
			}

			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			var state ScriptStateValue
			if err := json.Unmarshal(value, &state); err != nil {
				return fmt.Errorf("failed to unmarshal state %s: %w", key, err)
			}

			states[key] = &state
		}
		return nil
	})

	return states, err
}
