package storage

import (
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreateScript creates a new script with triggers
func (db *DB) CreateScript(name, description, scriptContent string, enabled bool, metadata datatypes.JSON, triggers []ScriptTrigger) (*Script, error) {
	if name == "" {
		return nil, fmt.Errorf("script name is required")
	}
	if scriptContent == "" {
		return nil, fmt.Errorf("script content is required")
	}

	script := &Script{
		Name:        name,
		Description: description,
		Content:     scriptContent,
		Enabled:     enabled,
		Metadata:    metadata,
		Triggers:    triggers,
	}

	// Create script
	if err := db.Create(script).Error; err != nil {
		return nil, fmt.Errorf("failed to create script: %w", err)
	}

	// GORM workaround: if enabled=false, explicitly update it
	// (GORM's default:true tag interferes with zero values)
	if !enabled {
		if err := db.Model(script).Update("enabled", false).Error; err != nil {
			return nil, fmt.Errorf("failed to set enabled=false: %w", err)
		}
	}

	return script, nil
}

// GetScript retrieves a script by ID with its triggers
func (db *DB) GetScript(id uint) (*Script, error) {
	var script Script
	if err := db.Preload("Triggers").First(&script, id).Error; err != nil {
		return nil, err
	}
	return &script, nil
}

// GetScriptByName retrieves a script by name with its triggers
func (db *DB) GetScriptByName(name string) (*Script, error) {
	var script Script
	if err := db.Preload("Triggers").Where("name = ?", name).First(&script).Error; err != nil {
		return nil, err
	}
	return &script, nil
}

// ListScripts returns all scripts with their triggers
func (db *DB) ListScripts() ([]Script, error) {
	var scripts []Script
	if err := db.Preload("Triggers").Find(&scripts).Error; err != nil {
		return nil, err
	}
	return scripts, nil
}

// ListScriptsPaginated returns paginated scripts with search and sorting
func (db *DB) ListScriptsPaginated(page, pageSize int, search, sortBy, sortOrder string) ([]Script, int64, error) {
	var scripts []Script
	var total int64

	query := db.Model(&Script{})

	// Apply search filter
	if search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?",
			"%"+search+"%", "%"+search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count scripts: %w", err)
	}

	// Apply sorting
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" || (sortOrder != "asc" && sortOrder != "desc") {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize)

	// Execute query with triggers preloaded
	if err := query.Preload("Triggers").Find(&scripts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list scripts: %w", err)
	}

	return scripts, total, nil
}

// UpdateScript updates a script's information and triggers
func (db *DB) UpdateScript(id uint, name, description, scriptContent string, enabled bool, metadata datatypes.JSON, triggers []ScriptTrigger) error {
	// Start transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Update script fields
		updates := map[string]interface{}{
			"name":        name,
			"description": description,
			"content":     scriptContent,
			"enabled":     enabled,
		}

		if metadata != nil {
			updates["metadata"] = metadata
		}

		result := tx.Model(&Script{}).Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to update script: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("script not found")
		}

		// Delete existing triggers
		if err := tx.Where("script_id = ?", id).Delete(&ScriptTrigger{}).Error; err != nil {
			return fmt.Errorf("failed to delete old triggers: %w", err)
		}

		// Create new triggers
		for i := range triggers {
			triggers[i].ScriptID = id
			if err := tx.Create(&triggers[i]).Error; err != nil {
				return fmt.Errorf("failed to create trigger: %w", err)
			}
		}

		return nil
	})
}

// DeleteScript deletes a script and cascades to triggers and logs
func (db *DB) DeleteScript(id uint) error {
	result := db.Delete(&Script{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete script: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("script not found")
	}

	return nil
}

// UpdateScriptEnabled updates only the enabled status of a script
func (db *DB) UpdateScriptEnabled(id uint, enabled bool) error {
	result := db.Model(&Script{}).Where("id = ?", id).Update("enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("failed to update script enabled status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("script not found")
	}

	return nil
}

// GetEnabledScriptsForTrigger retrieves all enabled scripts with matching triggers for a given event type and topic
// This is the key function called by the script hook
func (db *DB) GetEnabledScriptsForTrigger(triggerType, topic string) ([]Script, error) {
	var scripts []Script

	// Build query: script must be enabled, trigger must be enabled and match type
	query := db.Preload("Triggers", "type = ? AND enabled = ?", triggerType, true).
		Joins("JOIN script_triggers ON script_triggers.script_id = scripts.id").
		Where("scripts.enabled = ?", true).
		Where("script_triggers.type = ?", triggerType).
		Where("script_triggers.enabled = ?", true).
		Distinct()

	if err := query.Find(&scripts).Error; err != nil {
		return nil, fmt.Errorf("failed to get enabled scripts: %w", err)
	}

	// Filter by topic pattern (for on_publish and on_subscribe events)
	if topic != "" {
		filtered := make([]Script, 0)
		for _, script := range scripts {
			for _, trigger := range script.Triggers {
				if trigger.Type == triggerType && trigger.Enabled {
					// Empty topic filter matches all topics
					if trigger.Topic == "" || MatchTopic(trigger.Topic, topic) {
						filtered = append(filtered, script)
						break // Only add script once even if multiple triggers match
					}
				}
			}
		}
		scripts = filtered
	}

	// Sort by priority (lower = earlier)
	// Note: This is in-memory sorting. For large datasets, consider DB-level sorting
	// For now, keeping it simple since we expect relatively few scripts per trigger
	if len(scripts) > 1 {
		scripts = sortScriptsByPriority(scripts, triggerType)
	}

	return scripts, nil
}

// sortScriptsByPriority sorts scripts by their trigger priority (lower priority = earlier execution)
func sortScriptsByPriority(scripts []Script, triggerType string) []Script {
	// Create a slice of (script, minPriority) pairs
	type scriptPriority struct {
		script   Script
		priority int
	}

	pairs := make([]scriptPriority, 0, len(scripts))
	for _, script := range scripts {
		// Find minimum priority among matching triggers
		minPriority := 999999
		for _, trigger := range script.Triggers {
			if trigger.Type == triggerType && trigger.Enabled {
				if trigger.Priority < minPriority {
					minPriority = trigger.Priority
				}
			}
		}
		pairs = append(pairs, scriptPriority{script: script, priority: minPriority})
	}

	// Simple bubble sort (fine for small lists)
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].priority > pairs[j].priority {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// Extract sorted scripts
	sorted := make([]Script, len(pairs))
	for i, pair := range pairs {
		sorted[i] = pair.script
	}

	return sorted
}

// CreateProvisionedScript creates a new script marked as provisioned from config
func (db *DB) CreateProvisionedScript(name, description, scriptContent string, enabled bool, metadata datatypes.JSON, triggers []ScriptTrigger) (*Script, error) {
	script := &Script{
		Name:                  name,
		Description:           description,
		Content:               scriptContent,
		Enabled:               enabled,
		ProvisionedFromConfig: true,
		Metadata:              metadata,
		Triggers:              triggers,
	}

	// Create provisioned script
	if err := db.Create(script).Error; err != nil {
		return nil, fmt.Errorf("failed to create provisioned script: %w", err)
	}

	// GORM workaround: if enabled=false, explicitly update it
	if !enabled {
		if err := db.Model(script).Update("enabled", false).Error; err != nil {
			return nil, fmt.Errorf("failed to set enabled=false: %w", err)
		}
	}

	return script, nil
}

// UpdateProvisionedScript updates a provisioned script
func (db *DB) UpdateProvisionedScript(id uint, name, description, scriptContent string, enabled bool, metadata datatypes.JSON, triggers []ScriptTrigger) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Update script fields
		updates := map[string]interface{}{
			"name":                    name,
			"description":             description,
			"content":                 scriptContent,
			"enabled":                 enabled,
			"provisioned_from_config": true,
		}

		if metadata != nil {
			updates["metadata"] = metadata
		}

		result := tx.Model(&Script{}).Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("failed to update provisioned script: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("script not found")
		}

		// Delete existing triggers
		if err := tx.Where("script_id = ?", id).Delete(&ScriptTrigger{}).Error; err != nil {
			return fmt.Errorf("failed to delete old triggers: %w", err)
		}

		// Create new triggers
		for i := range triggers {
			triggers[i].ScriptID = id
			if err := tx.Create(&triggers[i]).Error; err != nil {
				return fmt.Errorf("failed to create trigger: %w", err)
			}
		}

		return nil
	})
}

// ListProvisionedScripts returns all scripts that were provisioned from config
func (db *DB) ListProvisionedScripts() ([]Script, error) {
	var scripts []Script
	if err := db.Preload("Triggers").Where("provisioned_from_config = ?", true).Find(&scripts).Error; err != nil {
		return nil, err
	}
	return scripts, nil
}

// DeleteProvisionedScripts deletes all scripts that were provisioned from config
func (db *DB) DeleteProvisionedScripts() error {
	result := db.Where("provisioned_from_config = ?", true).Delete(&Script{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete provisioned scripts: %w", result.Error)
	}
	return nil
}
