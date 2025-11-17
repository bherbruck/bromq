package storage

import (
	"fmt"
	"time"

	"gorm.io/datatypes"
)

// CreateScriptLog creates a new log entry for script execution
func (db *DB) CreateScriptLog(scriptID uint, triggerType, level, message string, context datatypes.JSON, executionTimeMs int) error {
	log := &ScriptLog{
		ScriptID:        scriptID,
		Type:            triggerType,
		Level:           level,
		Message:         message,
		Context:         context,
		ExecutionTimeMs: executionTimeMs,
	}

	if err := db.Create(log).Error; err != nil {
		return fmt.Errorf("failed to create script log: %w", err)
	}

	return nil
}

// ListScriptLogs returns all logs for a specific script, paginated and sorted by creation time (newest first)
func (db *DB) ListScriptLogs(scriptID uint, page, pageSize int, level string) ([]ScriptLog, int64, error) {
	var logs []ScriptLog
	var total int64

	query := db.Model(&ScriptLog{}).Where("script_id = ?", scriptID)

	// Filter by level if specified
	if level != "" {
		query = query.Where("level = ?", level)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count script logs: %w", err)
	}

	// Apply pagination and sorting (newest first)
	offset := (page - 1) * pageSize
	query = query.Order("created_at DESC").Offset(offset).Limit(pageSize)

	// Execute query
	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list script logs: %w", err)
	}

	return logs, total, nil
}

// ClearScriptLogs deletes all logs for a specific script
func (db *DB) ClearScriptLogs(scriptID uint) error {
	result := db.Where("script_id = ?", scriptID).Delete(&ScriptLog{})
	if result.Error != nil {
		return fmt.Errorf("failed to clear script logs: %w", result.Error)
	}
	return nil
}

// ClearScriptLogsBefore deletes logs older than a specified time
func (db *DB) ClearScriptLogsBefore(scriptID uint, before time.Time) error {
	result := db.Where("script_id = ? AND created_at < ?", scriptID, before).Delete(&ScriptLog{})
	if result.Error != nil {
		return fmt.Errorf("failed to clear old script logs: %w", result.Error)
	}
	return nil
}

// ClearAllScriptLogsBefore deletes all logs older than a specified time (for cleanup jobs)
func (db *DB) ClearAllScriptLogsBefore(before time.Time) error {
	result := db.Where("created_at < ?", before).Delete(&ScriptLog{})
	if result.Error != nil {
		return fmt.Errorf("failed to clear old logs: %w", result.Error)
	}
	return nil
}

// GetScriptLogStats returns statistics for a script's logs
func (db *DB) GetScriptLogStats(scriptID uint) (map[string]int64, error) {
	stats := make(map[string]int64)

	// Count by level
	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		var count int64
		if err := db.Model(&ScriptLog{}).Where("script_id = ? AND level = ?", scriptID, level).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count logs for level %s: %w", level, err)
		}
		stats[level] = count
	}

	// Total count
	var total int64
	if err := db.Model(&ScriptLog{}).Where("script_id = ?", scriptID).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count total logs: %w", err)
	}
	stats["total"] = total

	return stats, nil
}
