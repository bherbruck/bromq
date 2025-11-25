package provisioning

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github/bromq-dev/bromq/internal/config"
	"github/bromq-dev/bromq/internal/storage"
)

// Provision syncs the configuration file to the database
// This function is idempotent and can be run on every startup
func Provision(db *storage.DB, cfg *config.Config) error {
	slog.Info("Starting configuration provisioning",
		"users", len(cfg.Users),
		"acl_rules", len(cfg.ACLRules),
		"bridges", len(cfg.Bridges),
		"scripts", len(cfg.Scripts))

	// Step 1: Provision MQTT users
	userIDMap := make(map[string]uint) // username -> database ID
	for _, userCfg := range cfg.Users {
		userID, err := provisionUser(db, userCfg)
		if err != nil {
			return fmt.Errorf("failed to provision user '%s': %w", userCfg.Username, err)
		}
		userIDMap[userCfg.Username] = userID
		slog.Debug("Provisioned MQTT user", "username", userCfg.Username, "id", userID)
	}

	// Step 2: Provision ACL rules (smart diff-based approach)
	if err := syncACLRules(db, userIDMap, cfg.ACLRules); err != nil {
		return fmt.Errorf("failed to sync ACL rules: %w", err)
	}

	// Step 3: Provision bridges
	bridgeIDMap := make(map[string]uint) // bridge name -> database ID
	for _, bridgeCfg := range cfg.Bridges {
		bridgeID, err := provisionBridge(db, bridgeCfg)
		if err != nil {
			return fmt.Errorf("failed to provision bridge '%s': %w", bridgeCfg.Name, err)
		}
		bridgeIDMap[bridgeCfg.Name] = bridgeID
		slog.Debug("Provisioned bridge", "name", bridgeCfg.Name, "id", bridgeID)
	}

	// Step 4: Provision scripts
	scriptIDMap := make(map[string]uint) // script name -> database ID
	for _, scriptCfg := range cfg.Scripts {
		scriptID, err := provisionScript(db, scriptCfg)
		if err != nil {
			return fmt.Errorf("failed to provision script '%s': %w", scriptCfg.Name, err)
		}
		scriptIDMap[scriptCfg.Name] = scriptID
		slog.Debug("Provisioned script", "name", scriptCfg.Name, "id", scriptID)
	}

	// Clean up users that were provisioned but are no longer in config
	if err := cleanupOrphanedUsers(db, userIDMap); err != nil {
		slog.Warn("Failed to cleanup orphaned users", "error", err)
	}

	// Clean up bridges that were provisioned but are no longer in config
	if err := cleanupOrphanedBridges(db, bridgeIDMap); err != nil {
		slog.Warn("Failed to cleanup orphaned bridges", "error", err)
	}

	// Clean up scripts that were provisioned but are no longer in config
	if err := cleanupOrphanedScripts(db, scriptIDMap); err != nil {
		slog.Warn("Failed to cleanup orphaned scripts", "error", err)
	}

	slog.Info("Configuration provisioning completed successfully")
	return nil
}

// provisionUser creates or updates an MQTT user
func provisionUser(db *storage.DB, userCfg config.MQTTUserConfig) (uint, error) {
	// Check if user already exists
	existingUser, err := db.GetMQTTUserByUsername(userCfg.Username)
	if err == nil {
		// User exists - update password and metadata
		if err := db.UpdateMQTTUserPassword(existingUser.ID, userCfg.Password); err != nil {
			return 0, fmt.Errorf("failed to update password: %w", err)
		}

		// Convert metadata map to JSON
		var metadataJSON []byte
		if userCfg.Metadata != nil {
			metadataJSON, err = json.Marshal(userCfg.Metadata)
			if err != nil {
				return 0, fmt.Errorf("failed to marshal metadata: %w", err)
			}
		}

		if err := db.UpdateMQTTUser(existingUser.ID, userCfg.Username, userCfg.Description, metadataJSON); err != nil {
			return 0, fmt.Errorf("failed to update user: %w", err)
		}

		// Mark as provisioned
		if err := db.MarkAsProvisioned(existingUser.ID, true); err != nil {
			return 0, fmt.Errorf("failed to mark user as provisioned: %w", err)
		}

		return existingUser.ID, nil
	}

	// User doesn't exist - create new
	var metadataJSON []byte
	if userCfg.Metadata != nil {
		metadataJSON, err = json.Marshal(userCfg.Metadata)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	user, err := db.CreateMQTTUser(userCfg.Username, userCfg.Password, userCfg.Description, metadataJSON)
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	// Mark as provisioned
	if err := db.MarkAsProvisioned(user.ID, true); err != nil {
		return 0, fmt.Errorf("failed to mark new user as provisioned: %w", err)
	}

	return user.ID, nil
}

// syncACLRules intelligently syncs ACL rules - only modifies what changed
func syncACLRules(db *storage.DB, userIDMap map[string]uint, configRules []config.ACLRuleConfig) error {
	// Build map of config rules by user
	configRulesByUser := make(map[uint][]config.ACLRuleConfig)
	for _, ruleCfg := range configRules {
		userID, ok := userIDMap[ruleCfg.Username]
		if !ok {
			return fmt.Errorf("ACL rule references unknown user: %s", ruleCfg.Username)
		}
		configRulesByUser[userID] = append(configRulesByUser[userID], ruleCfg)
	}

	// Process each user in config
	for username, userID := range userIDMap {
		// Get existing provisioned rules from DB
		existingRules, err := db.GetACLRulesByMQTTUserID(userID)
		if err != nil {
			return fmt.Errorf("failed to get existing ACL rules for user '%s': %w", username, err)
		}

		// Filter to only provisioned rules
		provisionedRules := []storage.ACLRule{}
		for _, rule := range existingRules {
			if rule.ProvisionedFromConfig {
				provisionedRules = append(provisionedRules, rule)
			}
		}

		// Get config rules for this user (may be empty)
		configRules := configRulesByUser[userID]

		// Build map of existing rules: (topic, permission) -> rule
		existingMap := make(map[string]storage.ACLRule)
		for _, rule := range provisionedRules {
			key := rule.Topic + "|" + rule.Permission
			existingMap[key] = rule
		}

		// Build set of config rules
		configSet := make(map[string]config.ACLRuleConfig)
		for _, ruleCfg := range configRules {
			key := ruleCfg.Topic + "|" + ruleCfg.Permission
			configSet[key] = ruleCfg
		}

		// Find rules to delete (in DB but not in config)
		for key, existingRule := range existingMap {
			if _, inConfig := configSet[key]; !inConfig {
				slog.Debug("Deleting removed ACL rule", "username", username, "topic", existingRule.Topic, "permission", existingRule.Permission)
				if err := db.DeleteACLRule(existingRule.ID); err != nil {
					return fmt.Errorf("failed to delete ACL rule: %w", err)
				}
			}
		}

		// Find rules to create (in config but not in DB)
		for key, ruleCfg := range configSet {
			if _, exists := existingMap[key]; !exists {
				slog.Debug("Creating new ACL rule", "username", username, "topic", ruleCfg.Topic, "permission", ruleCfg.Permission)
				if err := db.CreateProvisionedACLRule(userID, ruleCfg.Topic, ruleCfg.Permission); err != nil {
					return fmt.Errorf("failed to create ACL rule: %w", err)
				}
			}
			// If rule exists with same values, no action needed (efficient!)
		}
	}

	return nil
}

// cleanupOrphanedUsers removes users that were provisioned but are no longer in config
func cleanupOrphanedUsers(db *storage.DB, currentUserMap map[string]uint) error {
	// Get all provisioned users from database
	provisionedUsers, err := db.ListProvisionedMQTTUsers()
	if err != nil {
		return fmt.Errorf("failed to list provisioned users: %w", err)
	}

	// Check which ones are no longer in config
	for _, user := range provisionedUsers {
		if _, exists := currentUserMap[user.Username]; !exists {
			// User was provisioned but is no longer in config - remove it
			slog.Info("Removing orphaned provisioned user", "username", user.Username, "id", user.ID)
			if err := db.DeleteMQTTUser(user.ID); err != nil {
				slog.Warn("Failed to delete orphaned user", "username", user.Username, "error", err)
			}
		}
	}

	return nil
}

// provisionBridge creates or updates a bridge with its topics
func provisionBridge(db *storage.DB, bridgeCfg config.BridgeConfig) (uint, error) {
	// Set defaults
	if bridgeCfg.Port == 0 {
		bridgeCfg.Port = 1883
	}
	if bridgeCfg.KeepAlive == 0 {
		bridgeCfg.KeepAlive = 60
	}
	if bridgeCfg.ConnectionTimeout == 0 {
		bridgeCfg.ConnectionTimeout = 30
	}
	if bridgeCfg.MQTTVersion == "" {
		bridgeCfg.MQTTVersion = "5" // Default to MQTT v5
	}

	// Convert metadata map to JSON
	var metadataJSON []byte
	var err error
	if bridgeCfg.Metadata != nil {
		metadataJSON, err = json.Marshal(bridgeCfg.Metadata)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Convert config topics to storage topics
	topics := make([]storage.BridgeTopic, len(bridgeCfg.Topics))
	for i, topicCfg := range bridgeCfg.Topics {
		topics[i] = storage.BridgeTopic{
			Local:     topicCfg.Local,
			Remote:    topicCfg.Remote,
			Direction: topicCfg.Direction,
			QoS:       byte(topicCfg.QoS),
		}
	}

	// Check if bridge already exists
	existingBridge, err := db.GetBridgeByName(bridgeCfg.Name)
	if err == nil {
		// Bridge exists - update it directly (bypass API protection since this is provisioning)
		// Update bridge configuration
		updates := map[string]interface{}{
			"name":                    bridgeCfg.Name,
			"host":                    bridgeCfg.Host,
			"port":                    bridgeCfg.Port,
			"username":                bridgeCfg.Username,
			"password":                bridgeCfg.Password,
			"client_id":               bridgeCfg.ClientID,
			"mqtt_version":            bridgeCfg.MQTTVersion,
			"clean_session":           bridgeCfg.CleanSession,
			"keep_alive":              bridgeCfg.KeepAlive,
			"connection_timeout":      bridgeCfg.ConnectionTimeout,
			"metadata":                metadataJSON,
			"provisioned_from_config": true,
		}
		if err := db.Model(&storage.Bridge{}).Where("id = ?", existingBridge.ID).Updates(updates).Error; err != nil {
			return 0, fmt.Errorf("failed to update bridge: %w", err)
		}

		// Update topics (delete old, create new)
		if err := db.Where("bridge_id = ?", existingBridge.ID).Delete(&storage.BridgeTopic{}).Error; err != nil {
			return 0, fmt.Errorf("failed to delete old topics: %w", err)
		}
		for i := range topics {
			topics[i].BridgeID = existingBridge.ID
		}
		if len(topics) > 0 {
			if err := db.Create(&topics).Error; err != nil {
				return 0, fmt.Errorf("failed to create new topics: %w", err)
			}
		}

		return existingBridge.ID, nil
	}

	// Bridge doesn't exist - create new
	bridge, err := db.CreateBridge(
		bridgeCfg.Name,
		bridgeCfg.Host,
		bridgeCfg.Port,
		bridgeCfg.Username,
		bridgeCfg.Password,
		bridgeCfg.ClientID,
		bridgeCfg.MQTTVersion,
		bridgeCfg.CleanSession,
		bridgeCfg.KeepAlive,
		bridgeCfg.ConnectionTimeout,
		metadataJSON,
		topics,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create bridge: %w", err)
	}

	// Mark as provisioned
	if err := db.MarkBridgeAsProvisioned(bridge.ID, true); err != nil {
		return 0, fmt.Errorf("failed to mark new bridge as provisioned: %w", err)
	}

	return bridge.ID, nil
}

// cleanupOrphanedBridges removes bridges that were provisioned but are no longer in config
func cleanupOrphanedBridges(db *storage.DB, currentBridgeMap map[string]uint) error {
	// Get all provisioned bridges from database
	provisionedBridges, err := db.ListProvisionedBridges()
	if err != nil {
		return fmt.Errorf("failed to list provisioned bridges: %w", err)
	}

	// Check which ones are no longer in config
	for _, bridge := range provisionedBridges {
		if _, exists := currentBridgeMap[bridge.Name]; !exists {
			// Bridge was provisioned but is no longer in config - remove it
			slog.Info("Removing orphaned provisioned bridge", "name", bridge.Name, "id", bridge.ID)
			if err := db.DeleteBridge(bridge.ID); err != nil {
				slog.Warn("Failed to delete orphaned bridge", "name", bridge.Name, "error", err)
			}
		}
	}

	return nil
}

// provisionScript creates or updates a script
func provisionScript(db *storage.DB, scriptCfg config.ScriptConfig) (uint, error) {
	// Load script content from file if specified
	scriptContent := scriptCfg.Content
	if scriptCfg.File != "" {
		content, err := os.ReadFile(scriptCfg.File)
		if err != nil {
			return 0, fmt.Errorf("failed to read script file '%s': %w", scriptCfg.File, err)
		}
		scriptContent = string(content)
	}

	// Convert metadata to JSON
	var metadataJSON []byte
	var err error
	if scriptCfg.Metadata != nil {
		metadataJSON, err = json.Marshal(scriptCfg.Metadata)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Convert triggers
	triggers := make([]storage.ScriptTrigger, len(scriptCfg.Triggers))
	for i, t := range scriptCfg.Triggers {
		triggers[i] = storage.ScriptTrigger{
			Type:     t.Type,
			Topic:    t.Topic,
			Priority: t.Priority,
			Enabled:  t.Enabled,
		}
	}

	// Check if script already exists
	existingScript, err := db.GetScriptByName(scriptCfg.Name)
	if err == nil {
		// Script exists - update it
		if err := db.UpdateProvisionedScript(
			existingScript.ID,
			scriptCfg.Name,
			scriptCfg.Description,
			scriptContent,
			scriptCfg.Enabled,
			metadataJSON,
			triggers,
		); err != nil {
			return 0, fmt.Errorf("failed to update script: %w", err)
		}
		return existingScript.ID, nil
	}

	// Script doesn't exist - create it
	script, err := db.CreateProvisionedScript(
		scriptCfg.Name,
		scriptCfg.Description,
		scriptContent,
		scriptCfg.Enabled,
		metadataJSON,
		triggers,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create script: %w", err)
	}

	return script.ID, nil
}

// cleanupOrphanedScripts removes scripts that were provisioned but are no longer in config
func cleanupOrphanedScripts(db *storage.DB, currentScriptMap map[string]uint) error {
	// Get all provisioned scripts
	provisionedScripts, err := db.ListProvisionedScripts()
	if err != nil {
		return fmt.Errorf("failed to list provisioned scripts: %w", err)
	}

	// Check each provisioned script
	for _, script := range provisionedScripts {
		if _, exists := currentScriptMap[script.Name]; !exists {
			// Script was provisioned but is no longer in config - remove it
			slog.Info("Removing orphaned provisioned script", "name", script.Name, "id", script.ID)
			if err := db.DeleteScript(script.ID); err != nil {
				slog.Warn("Failed to delete orphaned script", "name", script.Name, "error", err)
			}
		}
	}

	return nil
}
