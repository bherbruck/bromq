package script

import (
	"log/slog"
	"sync"

	"github/bromq-dev/bromq/internal/storage"
)

// ScriptCache caches enabled scripts in memory to avoid repeated database queries
// Scripts are loaded once and only reloaded when they change via API
type ScriptCache struct {
	db      *storage.DB
	scripts map[string][]storage.Script // Map: triggerType -> scripts
	mu      sync.RWMutex
}

// NewScriptCache creates a new script cache
func NewScriptCache(db *storage.DB) *ScriptCache {
	return &ScriptCache{
		db:      db,
		scripts: make(map[string][]storage.Script),
	}
}

// Load loads all enabled scripts from database into memory
func (c *ScriptCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get all enabled scripts with their triggers
	var scripts []storage.Script
	err := c.db.Preload("Triggers", "enabled = ?", true).
		Where("enabled = ?", true).
		Find(&scripts).Error
	if err != nil {
		return err
	}

	// Group by trigger type for fast lookup
	cache := make(map[string][]storage.Script)
	for _, script := range scripts {
		for _, trigger := range script.Triggers {
			if trigger.Enabled {
				cache[trigger.Type] = append(cache[trigger.Type], script)
			}
		}
	}

	c.scripts = cache

	// Count total triggers
	totalTriggers := 0
	for _, scripts := range cache {
		for _, script := range scripts {
			totalTriggers += len(script.Triggers)
		}
	}

	slog.Info("Script cache loaded",
		"scripts", len(scripts),
		"trigger_types", len(cache),
		"total_triggers", totalTriggers)

	return nil
}

// GetScriptsForTrigger returns cached scripts matching the trigger type and topic
func (c *ScriptCache) GetScriptsForTrigger(triggerType, topic string) []storage.Script {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scripts, ok := c.scripts[triggerType]
	if !ok {
		return nil
	}

	// If no topic filtering needed, return all scripts for this trigger type
	if topic == "" {
		return scripts
	}

	// Filter by topic pattern
	filtered := make([]storage.Script, 0, len(scripts))
	for _, script := range scripts {
		for _, trigger := range script.Triggers {
			if trigger.Type == triggerType && trigger.Enabled {
				// Empty topic filter matches all topics
				if trigger.Topic == "" || storage.MatchTopic(trigger.Topic, topic) {
					filtered = append(filtered, script)
					break // Only add script once even if multiple triggers match
				}
			}
		}
	}

	return filtered
}

// Reload reloads scripts from database (called when scripts change via API)
func (c *ScriptCache) Reload() error {
	slog.Debug("Reloading script cache")
	return c.Load()
}
