package storage

import (
	"fmt"
	"strings"
)

// ListACLRules returns all ACL rules
func (db *DB) ListACLRules() ([]ACLRule, error) {
	var rules []ACLRule
	err := db.Order("mqtt_user_id, topic_pattern").Find(&rules).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list ACL rules: %w", err)
	}
	return rules, nil
}

// GetACLRulesByMQTTUserID returns all ACL rules for a specific MQTT user
func (db *DB) GetACLRulesByMQTTUserID(mqttUserID int) ([]ACLRule, error) {
	var rules []ACLRule
	err := db.Where("mqtt_user_id = ?", mqttUserID).Order("topic_pattern").Find(&rules).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL rules: %w", err)
	}
	return rules, nil
}

// CreateACLRule creates a new ACL rule
func (db *DB) CreateACLRule(mqttUserID int, topicPattern, permission string) (*ACLRule, error) {
	// Validate permission
	if permission != "pub" && permission != "sub" && permission != "pubsub" {
		return nil, fmt.Errorf("invalid permission: must be 'pub', 'sub', or 'pubsub'")
	}

	// Verify MQTT user exists
	user, err := db.GetMQTTUser(mqttUserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("MQTT user not found")
	}

	// Create rule
	rule := ACLRule{
		MQTTUserID:   uint(mqttUserID),
		TopicPattern: topicPattern,
		Permission:   permission,
	}

	if err := db.Create(&rule).Error; err != nil {
		return nil, fmt.Errorf("failed to create ACL rule: %w", err)
	}

	return &rule, nil
}

// UpdateACLRule updates an existing ACL rule
func (db *DB) UpdateACLRule(id int, topicPattern, permission string) (*ACLRule, error) {
	// Validate permission
	if permission != "pub" && permission != "sub" && permission != "pubsub" {
		return nil, fmt.Errorf("invalid permission: must be 'pub', 'sub', or 'pubsub'")
	}

	// Find existing rule
	var rule ACLRule
	if err := db.First(&rule, id).Error; err != nil {
		return nil, fmt.Errorf("ACL rule not found")
	}

	// Update fields
	rule.TopicPattern = topicPattern
	rule.Permission = permission

	if err := db.Save(&rule).Error; err != nil {
		return nil, fmt.Errorf("failed to update ACL rule: %w", err)
	}

	return &rule, nil
}

// DeleteACLRule deletes an ACL rule by ID
func (db *DB) DeleteACLRule(id int) error {
	result := db.Delete(&ACLRule{}, id)

	if result.Error != nil {
		return fmt.Errorf("failed to delete ACL rule: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("ACL rule not found")
	}

	return nil
}

// CheckACL checks if an MQTT user has permission for a specific topic and action
// Note: This is for MQTT users only. Admin users (dashboard) don't use MQTT ACL checks.
// Supports dynamic placeholders: ${username} and ${clientid}
func (db *DB) CheckACL(username, clientID, topic, action string) (bool, error) {
	// Get MQTT user
	user, err := db.GetMQTTUserByUsername(username)
	if err != nil {
		// If user not found, deny access (not an error)
		if err.Error() == "record not found" {
			return false, nil
		}
		return false, err
	}
	if user == nil {
		return false, nil // User not found
	}

	// Get user's ACL rules
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		return false, err
	}

	// Check if any rule matches the topic
	for _, rule := range rules {
		// Replace placeholders in the pattern before matching
		expandedPattern := replacePlaceholders(rule.TopicPattern, username, clientID)

		if matchTopic(expandedPattern, topic) {
			// Check if permission matches action
			switch action {
			case "pub":
				if rule.Permission == "pub" || rule.Permission == "pubsub" {
					return true, nil
				}
			case "sub":
				if rule.Permission == "sub" || rule.Permission == "pubsub" {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// replacePlaceholders replaces dynamic placeholders in topic patterns
// Supports: ${username} and ${clientid}
func replacePlaceholders(pattern, username, clientID string) string {
	result := pattern
	result = strings.ReplaceAll(result, "${username}", username)
	result = strings.ReplaceAll(result, "${clientid}", clientID)
	return result
}

// matchTopic checks if a topic matches a pattern with MQTT wildcards (+ and #)
func matchTopic(pattern, topic string) bool {
	patternLevels := strings.Split(pattern, "/")
	topicLevels := strings.Split(topic, "/")

	pLen := len(patternLevels)
	tLen := len(topicLevels)

	for i := 0; i < pLen; i++ {
		// Multi-level wildcard (#) must be last and matches everything
		if patternLevels[i] == "#" {
			return i == pLen-1
		}

		// Check if we've run out of topic levels
		if i >= tLen {
			return false
		}

		// Single-level wildcard (+) matches any single level
		if patternLevels[i] == "+" {
			continue
		}

		// Exact match required
		if patternLevels[i] != topicLevels[i] {
			return false
		}
	}

	// If pattern has no wildcard at end, lengths must match
	return pLen == tLen
}
