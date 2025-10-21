package storage

import (
	"fmt"
	"strings"
)

type ACLRule struct {
	ID           int    `json:"id"`
	UserID       int    `json:"user_id"`
	TopicPattern string `json:"topic_pattern"`
	Permission   string `json:"permission"` // "pub", "sub", or "pubsub"
}

// ListACLRules returns all ACL rules
func (db *DB) ListACLRules() ([]ACLRule, error) {
	rows, err := db.Query("SELECT id, user_id, topic_pattern, permission FROM acl_rules ORDER BY user_id, topic_pattern")
	if err != nil {
		return nil, fmt.Errorf("failed to list ACL rules: %w", err)
	}
	defer rows.Close()

	var rules []ACLRule
	for rows.Next() {
		var rule ACLRule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.TopicPattern, &rule.Permission); err != nil {
			return nil, fmt.Errorf("failed to scan ACL rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// GetACLRulesByUserID returns all ACL rules for a specific user
func (db *DB) GetACLRulesByUserID(userID int) ([]ACLRule, error) {
	rows, err := db.Query(
		"SELECT id, user_id, topic_pattern, permission FROM acl_rules WHERE user_id = ? ORDER BY topic_pattern",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL rules: %w", err)
	}
	defer rows.Close()

	var rules []ACLRule
	for rows.Next() {
		var rule ACLRule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.TopicPattern, &rule.Permission); err != nil {
			return nil, fmt.Errorf("failed to scan ACL rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// CreateACLRule creates a new ACL rule
func (db *DB) CreateACLRule(userID int, topicPattern, permission string) (*ACLRule, error) {
	// Validate permission
	if permission != "pub" && permission != "sub" && permission != "pubsub" {
		return nil, fmt.Errorf("invalid permission: must be 'pub', 'sub', or 'pubsub'")
	}

	// Verify user exists
	user, err := db.GetUser(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	result, err := db.Exec(
		"INSERT INTO acl_rules (user_id, topic_pattern, permission) VALUES (?, ?, ?)",
		userID, topicPattern, permission,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACL rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL rule ID: %w", err)
	}

	return &ACLRule{
		ID:           int(id),
		UserID:       userID,
		TopicPattern: topicPattern,
		Permission:   permission,
	}, nil
}

// DeleteACLRule deletes an ACL rule by ID
func (db *DB) DeleteACLRule(id int) error {
	result, err := db.Exec("DELETE FROM acl_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete ACL rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("ACL rule not found")
	}

	return nil
}

// CheckACL checks if a user has permission for a specific topic and action
func (db *DB) CheckACL(username, topic, action string) (bool, error) {
	// Get user
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil // User not found
	}

	// Admin users have full access
	if user.Role == "admin" {
		return true, nil
	}

	// Get user's ACL rules
	rules, err := db.GetACLRulesByUserID(user.ID)
	if err != nil {
		return false, err
	}

	// Check if any rule matches the topic
	for _, rule := range rules {
		if matchTopic(rule.TopicPattern, topic) {
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
