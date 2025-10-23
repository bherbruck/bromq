package storage

import (
	"testing"
)

func TestCreateACLRule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user := createTestMQTTUser(t, db, "testuser", "password123", "Test MQTT user")

	tests := []struct {
		name         string
		userID       uint
		topicPattern string
		permission   string
		wantErr      bool
	}{
		{
			name:         "create pub rule",
			userID:       user.ID,
			topicPattern: "devices/+/telemetry",
			permission:   "pub",
			wantErr:      false,
		},
		{
			name:         "create sub rule",
			userID:       user.ID,
			topicPattern: "commands/#",
			permission:   "sub",
			wantErr:      false,
		},
		{
			name:         "create pubsub rule",
			userID:       user.ID,
			topicPattern: "chat/+/messages",
			permission:   "pubsub",
			wantErr:      false,
		},
		{
			name:         "create rule with invalid permission",
			userID:       user.ID,
			topicPattern: "test/topic",
			permission:   "readwrite",
			wantErr:      true,
		},
		{
			name:         "create rule for non-existent user",
			userID:       999999,
			topicPattern: "test/topic",
			permission:   "pub",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := db.CreateACLRule(int(tt.userID), tt.topicPattern, tt.permission)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateACLRule() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateACLRule() unexpected error: %v", err)
			}

			if rule.MQTTUserID != tt.userID {
				t.Errorf("CreateACLRule() userID = %v, want %v", rule.MQTTUserID, tt.userID)
			}

			if rule.TopicPattern != tt.topicPattern {
				t.Errorf("CreateACLRule() topicPattern = %v, want %v", rule.TopicPattern, tt.topicPattern)
			}

			if rule.Permission != tt.permission {
				t.Errorf("CreateACLRule() permission = %v, want %v", rule.Permission, tt.permission)
			}

			if rule.ID == 0 {
				t.Errorf("CreateACLRule() ID should not be 0")
			}
		})
	}
}

func TestListACLRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user1 := createTestMQTTUser(t, db, "user1", "password123", "MQTT user 1")
	user2 := createTestMQTTUser(t, db, "user2", "password123", "MQTT user 2")

	// Create test rules
	createTestACLRule(t, db, user1.ID, "devices/+/telemetry", "pub")
	createTestACLRule(t, db, user1.ID, "commands/#", "sub")
	createTestACLRule(t, db, user2.ID, "sensors/#", "pubsub")

	rules, err := db.ListACLRules()
	if err != nil {
		t.Fatalf("ListACLRules() unexpected error: %v", err)
	}

	if len(rules) != 3 {
		t.Errorf("ListACLRules() returned %d rules, want 3", len(rules))
	}
}

func TestGetACLRulesByMQTTUserID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user1 := createTestMQTTUser(t, db, "user1", "password123", "MQTT user 1")
	user2 := createTestMQTTUser(t, db, "user2", "password123", "MQTT user 2")

	// Create test rules
	createTestACLRule(t, db, user1.ID, "devices/+/telemetry", "pub")
	createTestACLRule(t, db, user1.ID, "commands/#", "sub")
	createTestACLRule(t, db, user2.ID, "sensors/#", "pubsub")

	tests := []struct {
		name      string
		userID    uint
		wantCount int
	}{
		{
			name:      "get rules for user1",
			userID:    user1.ID,
			wantCount: 2,
		},
		{
			name:      "get rules for user2",
			userID:    user2.ID,
			wantCount: 1,
		},
		{
			name:      "get rules for user with no rules",
			userID:    999999,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := db.GetACLRulesByMQTTUserID(int(tt.userID))
			if err != nil {
				t.Fatalf("GetACLRulesByMQTTUserID() unexpected error: %v", err)
			}

			if len(rules) != tt.wantCount {
				t.Errorf("GetACLRulesByMQTTUserID() returned %d rules, want %d", len(rules), tt.wantCount)
			}

			// Verify all rules belong to the correct user
			for _, rule := range rules {
				if rule.MQTTUserID != tt.userID {
					t.Errorf("GetACLRulesByMQTTUserID() rule userID = %v, want %v", rule.MQTTUserID, tt.userID)
				}
			}
		})
	}
}

func TestDeleteACLRule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user := createTestMQTTUser(t, db, "testuser", "password123", "Test MQTT user")

	tests := []struct {
		name    string
		setup   func() uint // returns rule ID to delete
		wantErr bool
	}{
		{
			name: "delete existing rule",
			setup: func() uint {
				rule := createTestACLRule(t, db, user.ID, "test/topic", "pub")
				return rule.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existent rule",
			setup: func() uint {
				return 999999
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := db.DeleteACLRule(int(id))

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteACLRule() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteACLRule() unexpected error: %v", err)
			}
		})
	}
}

func TestCheckACL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test MQTT user
	regularUser := createTestMQTTUser(t, db, "regularuser", "password123", "Regular MQTT user")

	// Create ACL rules for regular user
	createTestACLRule(t, db, regularUser.ID, "devices/+/telemetry", "pub")
	createTestACLRule(t, db, regularUser.ID, "commands/#", "sub")
	createTestACLRule(t, db, regularUser.ID, "chat/room1", "pubsub")

	tests := []struct {
		name         string
		username     string
		topic        string
		action       string
		wantAllowed  bool
		wantErr      bool
	}{
		// Regular user - publish tests
		{
			name:        "regular user can publish to matching pattern",
			username:    "regularuser",
			topic:       "devices/sensor1/telemetry",
			action:      "pub",
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "regular user cannot publish to non-matching topic",
			username:    "regularuser",
			topic:       "devices/sensor1/status",
			action:      "pub",
			wantAllowed: false,
			wantErr:     false,
		},
		{
			name:        "regular user cannot publish to subscribe-only topic",
			username:    "regularuser",
			topic:       "commands/device1",
			action:      "pub",
			wantAllowed: false,
			wantErr:     false,
		},

		// Regular user - subscribe tests
		{
			name:        "regular user can subscribe to wildcard pattern",
			username:    "regularuser",
			topic:       "commands/device1/start",
			action:      "sub",
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "regular user cannot subscribe to publish-only topic",
			username:    "regularuser",
			topic:       "devices/sensor1/telemetry",
			action:      "sub",
			wantAllowed: false,
			wantErr:     false,
		},

		// Regular user - pubsub tests
		{
			name:        "regular user can publish to pubsub topic",
			username:    "regularuser",
			topic:       "chat/room1",
			action:      "pub",
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "regular user can subscribe to pubsub topic",
			username:    "regularuser",
			topic:       "chat/room1",
			action:      "sub",
			wantAllowed: true,
			wantErr:     false,
		},

		// Non-existent user
		{
			name:        "non-existent user denied",
			username:    "nonexistent",
			topic:       "any/topic",
			action:      "pub",
			wantAllowed: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := db.CheckACL(tt.username, tt.topic, tt.action)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CheckACL() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("CheckACL() unexpected error: %v", err)
			}

			if allowed != tt.wantAllowed {
				t.Errorf("CheckACL() allowed = %v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestMatchTopic(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		topic   string
		want    bool
	}{
		// Exact matches
		{
			name:    "exact match",
			pattern: "devices/sensor1/telemetry",
			topic:   "devices/sensor1/telemetry",
			want:    true,
		},
		{
			name:    "exact mismatch",
			pattern: "devices/sensor1/telemetry",
			topic:   "devices/sensor2/telemetry",
			want:    false,
		},

		// Single-level wildcard (+)
		{
			name:    "single-level wildcard match",
			pattern: "devices/+/telemetry",
			topic:   "devices/sensor1/telemetry",
			want:    true,
		},
		{
			name:    "single-level wildcard match different value",
			pattern: "devices/+/telemetry",
			topic:   "devices/sensor2/telemetry",
			want:    true,
		},
		{
			name:    "single-level wildcard level mismatch",
			pattern: "devices/+/telemetry",
			topic:   "devices/sensor1/status",
			want:    false,
		},
		{
			name:    "single-level wildcard too many levels",
			pattern: "devices/+/telemetry",
			topic:   "devices/sensor1/room1/telemetry",
			want:    false,
		},
		{
			name:    "multiple single-level wildcards",
			pattern: "+/+/telemetry",
			topic:   "devices/sensor1/telemetry",
			want:    true,
		},

		// Multi-level wildcard (#)
		{
			name:    "multi-level wildcard match",
			pattern: "devices/#",
			topic:   "devices/sensor1/telemetry",
			want:    true,
		},
		{
			name:    "multi-level wildcard match deep",
			pattern: "devices/#",
			topic:   "devices/sensor1/room1/temperature",
			want:    true,
		},
		{
			name:    "multi-level wildcard match single level",
			pattern: "devices/#",
			topic:   "devices/sensor1",
			want:    true,
		},
		{
			name:    "multi-level wildcard mismatch",
			pattern: "devices/#",
			topic:   "sensors/sensor1/telemetry",
			want:    false,
		},
		{
			name:    "multi-level wildcard at root",
			pattern: "#",
			topic:   "any/deep/topic/structure",
			want:    true,
		},

		// Combined wildcards
		{
			name:    "combined single and multi-level wildcards",
			pattern: "devices/+/#",
			topic:   "devices/sensor1/room1/temperature",
			want:    true,
		},
		{
			name:    "combined wildcards mismatch",
			pattern: "devices/+/#",
			topic:   "sensors/sensor1/room1",
			want:    false,
		},

		// Edge cases
		{
			name:    "empty topic and pattern",
			pattern: "",
			topic:   "",
			want:    true,
		},
		{
			name:    "pattern longer than topic",
			pattern: "devices/sensor1/room1/telemetry",
			topic:   "devices/sensor1",
			want:    false,
		},
		{
			name:    "topic longer than pattern without wildcard",
			pattern: "devices/sensor1",
			topic:   "devices/sensor1/telemetry",
			want:    false,
		},
		{
			name:    "multi-level wildcard not at end is invalid",
			pattern: "devices/#/telemetry",
			topic:   "devices/sensor1/telemetry",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchTopic(tt.pattern, tt.topic)
			if got != tt.want {
				t.Errorf("matchTopic(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
			}
		})
	}
}

func TestDeleteUserCascadesACLRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a user with ACL rules
	user := createTestMQTTUser(t, db, "testuser", "password123", "Test MQTT user")
	createTestACLRule(t, db, user.ID, "devices/+/telemetry", "pub")
	createTestACLRule(t, db, user.ID, "commands/#", "sub")

	// Verify rules exist
	rulesBefore, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("GetACLRulesByMQTTUserID() before delete failed: %v", err)
	}
	if len(rulesBefore) != 2 {
		t.Fatalf("Expected 2 rules before delete, got %d", len(rulesBefore))
	}

	// Delete the user
	err = db.DeleteMQTTUser(int(user.ID))
	if err != nil {
		t.Fatalf("DeleteMQTTUser() failed: %v", err)
	}

	// Verify ACL rules are also deleted (cascade)
	rulesAfter, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("GetACLRulesByMQTTUserID() after delete failed: %v", err)
	}
	if len(rulesAfter) != 0 {
		t.Errorf("Expected 0 rules after user deletion (cascade), got %d", len(rulesAfter))
	}
}

func TestDuplicateACLRulePrevention(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user := createTestMQTTUser(t, db, "testuser", "password123", "Test MQTT user")

	// Create first ACL rule
	_, err := db.CreateACLRule(int(user.ID), "sensor/+/temp", "pub")
	if err != nil {
		t.Fatalf("CreateACLRule() first call failed: %v", err)
	}

	// Try to create duplicate ACL rule (same user, same topic pattern)
	_, err = db.CreateACLRule(int(user.ID), "sensor/+/temp", "sub")
	if err == nil {
		t.Error("CreateACLRule() should have failed for duplicate user+topic_pattern but succeeded")
	}

	// Verify only one rule exists
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("GetACLRulesByMQTTUserID() failed: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule after duplicate attempt, got %d", len(rules))
	}

	// Verify different user with same topic pattern is allowed
	user2 := createTestMQTTUser(t, db, "testuser2", "password123", "Test MQTT user 2")
	_, err = db.CreateACLRule(int(user2.ID), "sensor/+/temp", "pub")
	if err != nil {
		t.Errorf("CreateACLRule() should allow same topic for different user but failed: %v", err)
	}

	// Verify same user with different topic pattern is allowed
	_, err = db.CreateACLRule(int(user.ID), "sensor/+/humidity", "pub")
	if err != nil {
		t.Errorf("CreateACLRule() should allow different topic for same user but failed: %v", err)
	}
}
