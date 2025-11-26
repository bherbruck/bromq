package badgerstore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// ScriptLogEntry represents a script execution log entry in BadgerDB
type ScriptLogEntry struct {
	ID              string                 `json:"id"`               // Format: timestamp_nanoseconds
	ScriptID        uint                   `json:"script_id"`
	Type            string                 `json:"type"`             // Trigger type: on_publish, on_connect, etc.
	Level           string                 `json:"level"`            // debug, info, warn, error
	Message         string                 `json:"message"`
	Context         map[string]interface{} `json:"context,omitempty"` // Client ID, topic, etc.
	ExecutionTimeMs int                    `json:"execution_time_ms"`
	CreatedAt       time.Time              `json:"created_at"`
}

// SaveScriptLog stores a script execution log entry
func (b *BadgerStore) SaveScriptLog(scriptID uint, triggerType, level, message string, context map[string]interface{}, executionTimeMs int) error {
	now := time.Now()

	// Generate unique ID: timestamp in nanoseconds
	id := fmt.Sprintf("%d", now.UnixNano())

	entry := ScriptLogEntry{
		ID:              id,
		ScriptID:        scriptID,
		Type:            triggerType,
		Level:           level,
		Message:         message,
		Context:         context,
		ExecutionTimeMs: executionTimeMs,
		CreatedAt:       now,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal script log: %w", err)
	}

	// Key format: log:{scriptID}:{timestamp_ns}
	key := fmt.Sprintf("log:%d:%s", scriptID, id)
	return b.Set(key, data, 0) // No TTL - managed by retention policy
}

// ListScriptLogs retrieves logs for a specific script with pagination and filtering
// Returns logs sorted by created_at DESC (newest first)
func (b *BadgerStore) ListScriptLogs(scriptID uint, page, pageSize int, levelFilter string) ([]ScriptLogEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	prefix := fmt.Sprintf("log:%d:", scriptID)
	var allLogs []ScriptLogEntry
	var total int64

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			value, err := it.Item().ValueCopy(nil)
			if err != nil {
				return err
			}

			var entry ScriptLogEntry
			if err := json.Unmarshal(value, &entry); err != nil {
				return fmt.Errorf("failed to unmarshal script log: %w", err)
			}

			// Apply level filter if specified
			if levelFilter != "" && entry.Level != levelFilter {
				continue
			}

			allLogs = append(allLogs, entry)
			total++
		}
		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	// Sort by created_at DESC (newest first)
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].CreatedAt.After(allLogs[j].CreatedAt)
	})

	// Apply pagination
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= len(allLogs) {
		return []ScriptLogEntry{}, total, nil
	}

	if end > len(allLogs) {
		end = len(allLogs)
	}

	return allLogs[start:end], total, nil
}

// GetScriptLog retrieves a single log entry by ID and script ID
func (b *BadgerStore) GetScriptLog(scriptID uint, logID string) (*ScriptLogEntry, error) {
	key := fmt.Sprintf("log:%d:%s", scriptID, logID)
	data, err := b.Get(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil // Not found
	}

	var entry ScriptLogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal script log: %w", err)
	}

	return &entry, nil
}

// ClearScriptLogs deletes all logs for a specific script
func (b *BadgerStore) ClearScriptLogs(scriptID uint) error {
	prefix := fmt.Sprintf("log:%d:", scriptID)
	return b.DeletePrefix(prefix)
}

// ClearScriptLogsBefore deletes logs older than a specified time for a specific script
func (b *BadgerStore) ClearScriptLogsBefore(scriptID uint, before time.Time) error {
	prefix := fmt.Sprintf("log:%d:", scriptID)
	var keysToDelete []string

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // Only need keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().KeyCopy(nil))

			// Extract timestamp from key (format: log:{scriptID}:{timestamp_ns})
			parts := strings.Split(key, ":")
			if len(parts) != 3 {
				continue
			}

			timestampNs, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				continue
			}

			createdAt := time.Unix(0, timestampNs)
			if createdAt.Before(before) {
				keysToDelete = append(keysToDelete, key)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Delete keys in a write transaction
	return b.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearAllScriptLogsBefore deletes all logs older than a specified time (for cleanup jobs)
func (b *BadgerStore) ClearAllScriptLogsBefore(before time.Time) error {
	prefix := "log:"
	var keysToDelete []string

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // Only need keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().KeyCopy(nil))

			// Extract timestamp from key (format: log:{scriptID}:{timestamp_ns})
			parts := strings.Split(key, ":")
			if len(parts) != 3 {
				continue
			}

			timestampNs, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				continue
			}

			createdAt := time.Unix(0, timestampNs)
			if createdAt.Before(before) {
				keysToDelete = append(keysToDelete, key)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Delete keys in a write transaction
	return b.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetScriptLogStats returns statistics for a script's logs
func (b *BadgerStore) GetScriptLogStats(scriptID uint) (map[string]int64, error) {
	stats := map[string]int64{
		"debug": 0,
		"info":  0,
		"warn":  0,
		"error": 0,
		"total": 0,
	}

	prefix := fmt.Sprintf("log:%d:", scriptID)

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			value, err := it.Item().ValueCopy(nil)
			if err != nil {
				return err
			}

			var entry ScriptLogEntry
			if err := json.Unmarshal(value, &entry); err != nil {
				return fmt.Errorf("failed to unmarshal script log: %w", err)
			}

			stats[entry.Level]++
			stats["total"]++
		}
		return nil
	})

	return stats, err
}

// CountScriptLogs returns the total number of logs for a script
func (b *BadgerStore) CountScriptLogs(scriptID uint) (int64, error) {
	prefix := fmt.Sprintf("log:%d:", scriptID)
	var count int64

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		opts.PrefetchValues = false // Only need keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})

	return count, err
}
