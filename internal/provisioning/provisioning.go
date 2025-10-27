package provisioning

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github/bherbruck/mqtt-server/internal/config"
	"github/bherbruck/mqtt-server/internal/storage"
)

// Provision syncs the configuration file to the database
// This function is idempotent and can be run on every startup
func Provision(db *storage.DB, cfg *config.Config) error {
	slog.Info("Starting configuration provisioning", "users", len(cfg.Users), "acl_rules", len(cfg.ACLRules))

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

	// Clean up users that were provisioned but are no longer in config
	if err := cleanupOrphanedUsers(db, userIDMap); err != nil {
		slog.Warn("Failed to cleanup orphaned users", "error", err)
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
		if err := db.UpdateMQTTUserPassword(int(existingUser.ID), userCfg.Password); err != nil {
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

		if err := db.UpdateMQTTUser(int(existingUser.ID), userCfg.Username, userCfg.Description, metadataJSON); err != nil {
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

// deleteProvisionedACLRules deletes all provisioned ACL rules for a user
func deleteProvisionedACLRules(db *storage.DB, userID uint) error {
	return db.DeleteProvisionedACLRules(userID)
}

// provisionACLRule creates a new ACL rule marked as provisioned
func provisionACLRule(db *storage.DB, userID uint, ruleCfg config.ACLRuleConfig) error {
	return db.CreateProvisionedACLRule(userID, ruleCfg.TopicPattern, ruleCfg.Permission)
}

// syncACLRules intelligently syncs ACL rules - only modifies what changed
func syncACLRules(db *storage.DB, userIDMap map[string]uint, configRules []config.ACLRuleConfig) error {
	// Build map of config rules by user
	configRulesByUser := make(map[uint][]config.ACLRuleConfig)
	for _, ruleCfg := range configRules {
		userID, ok := userIDMap[ruleCfg.MQTTUsername]
		if !ok {
			return fmt.Errorf("ACL rule references unknown user: %s", ruleCfg.MQTTUsername)
		}
		configRulesByUser[userID] = append(configRulesByUser[userID], ruleCfg)
	}

	// Process each user in config
	for username, userID := range userIDMap {
		// Get existing provisioned rules from DB
		existingRules, err := db.GetACLRulesByMQTTUserID(int(userID))
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

		// Build map of existing rules: (topic_pattern, permission) -> rule
		existingMap := make(map[string]storage.ACLRule)
		for _, rule := range provisionedRules {
			key := rule.TopicPattern + "|" + rule.Permission
			existingMap[key] = rule
		}

		// Build set of config rules
		configSet := make(map[string]config.ACLRuleConfig)
		for _, ruleCfg := range configRules {
			key := ruleCfg.TopicPattern + "|" + ruleCfg.Permission
			configSet[key] = ruleCfg
		}

		// Find rules to delete (in DB but not in config)
		for key, existingRule := range existingMap {
			if _, inConfig := configSet[key]; !inConfig {
				slog.Debug("Deleting removed ACL rule", "username", username, "topic", existingRule.TopicPattern, "permission", existingRule.Permission)
				if err := db.DeleteACLRule(int(existingRule.ID)); err != nil {
					return fmt.Errorf("failed to delete ACL rule: %w", err)
				}
			}
		}

		// Find rules to create (in config but not in DB)
		for key, ruleCfg := range configSet {
			if _, exists := existingMap[key]; !exists {
				slog.Debug("Creating new ACL rule", "username", username, "topic", ruleCfg.TopicPattern, "permission", ruleCfg.Permission)
				if err := db.CreateProvisionedACLRule(userID, ruleCfg.TopicPattern, ruleCfg.Permission); err != nil {
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
			if err := db.DeleteMQTTUser(int(user.ID)); err != nil {
				slog.Warn("Failed to delete orphaned user", "username", user.Username, "error", err)
			}
		}
	}

	return nil
}
