package storage

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// GetScriptState retrieves a state value by key
func (db *DB) GetScriptState(key string) (*ScriptState, error) {
	var state ScriptState
	if err := db.Where("key = ?", key).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

// SetScriptState creates or updates a state value
func (db *DB) SetScriptState(key string, scriptID *uint, value []byte, expiresAt *time.Time) error {
	state := &ScriptState{
		Key:       key,
		ScriptID:  scriptID,
		Value:     value,
		ExpiresAt: expiresAt,
	}

	// Use GORM's Save to insert or update
	if err := db.Save(state).Error; err != nil {
		return fmt.Errorf("failed to set script state: %w", err)
	}

	return nil
}

// DeleteScriptState deletes a state value by key
func (db *DB) DeleteScriptState(key string) error {
	result := db.Where("key = ?", key).Delete(&ScriptState{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete script state: %w", result.Error)
	}
	return nil
}

// ListScriptStateKeys returns all keys for a specific script (or global if scriptID is nil)
func (db *DB) ListScriptStateKeys(scriptID *uint) ([]string, error) {
	var keys []string

	query := db.Model(&ScriptState{}).Select("key")
	if scriptID == nil {
		// Global state (script_id IS NULL)
		query = query.Where("script_id IS NULL")
	} else {
		// Script-specific state
		query = query.Where("script_id = ?", *scriptID)
	}

	if err := query.Pluck("key", &keys).Error; err != nil {
		return nil, fmt.Errorf("failed to list script state keys: %w", err)
	}

	return keys, nil
}

// DeleteScriptStates deletes all state values for a specific script
func (db *DB) DeleteScriptStates(scriptID uint) error {
	result := db.Where("script_id = ?", scriptID).Delete(&ScriptState{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete script states: %w", result.Error)
	}
	return nil
}

// ExpireScriptStates deletes all expired state entries
func (db *DB) ExpireScriptStates() error {
	now := time.Now()
	result := db.Where("expires_at IS NOT NULL AND expires_at < ?", now).Delete(&ScriptState{})
	if result.Error != nil {
		return fmt.Errorf("failed to expire script states: %w", result.Error)
	}
	return nil
}

// GetScriptStatesByPrefix returns all state entries matching a key prefix (useful for cleanup)
func (db *DB) GetScriptStatesByPrefix(prefix string) ([]ScriptState, error) {
	var states []ScriptState
	if err := db.Where("key LIKE ?", prefix+"%").Find(&states).Error; err != nil {
		return nil, fmt.Errorf("failed to get states by prefix: %w", err)
	}
	return states, nil
}

// BatchSetScriptState sets multiple state values in a transaction (for flush operations)
func (db *DB) BatchSetScriptState(states []ScriptState) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for i := range states {
			if err := tx.Save(&states[i]).Error; err != nil {
				return fmt.Errorf("failed to set state %s: %w", states[i].Key, err)
			}
		}
		return nil
	})
}

// CountScriptStates returns the number of state entries for a script
func (db *DB) CountScriptStates(scriptID *uint) (int64, error) {
	var count int64
	query := db.Model(&ScriptState{})
	if scriptID == nil {
		query = query.Where("script_id IS NULL")
	} else {
		query = query.Where("script_id = ?", *scriptID)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count script states: %w", err)
	}

	return count, nil
}
