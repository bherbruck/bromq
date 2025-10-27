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
		clientID     string
		topic        string
		action       string
		wantAllowed  bool
		wantErr      bool
	}{
		// Regular user - publish tests
		{
			name:        "regular user can publish to matching pattern",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "devices/sensor1/telemetry",
			action:      "pub",
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "regular user cannot publish to non-matching topic",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "devices/sensor1/status",
			action:      "pub",
			wantAllowed: false,
			wantErr:     false,
		},
		{
			name:        "regular user cannot publish to subscribe-only topic",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "commands/device1",
			action:      "pub",
			wantAllowed: false,
			wantErr:     false,
		},

		// Regular user - subscribe tests
		{
			name:        "regular user can subscribe to wildcard pattern",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "commands/device1/start",
			action:      "sub",
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "regular user cannot subscribe to publish-only topic",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "devices/sensor1/telemetry",
			action:      "sub",
			wantAllowed: false,
			wantErr:     false,
		},

		// Regular user - pubsub tests
		{
			name:        "regular user can publish to pubsub topic",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "chat/room1",
			action:      "pub",
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name:        "regular user can subscribe to pubsub topic",
			username:    "regularuser",
			clientID:    "client1",
			topic:       "chat/room1",
			action:      "sub",
			wantAllowed: true,
			wantErr:     false,
		},

		// Non-existent user
		{
			name:        "non-existent user denied",
			username:    "nonexistent",
			clientID:    "client1",
			topic:       "any/topic",
			action:      "pub",
			wantAllowed: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := db.CheckACL(tt.username, tt.clientID, tt.topic, tt.action)

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

func TestReplacePlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		username string
		clientID string
		want     string
	}{
		{
			name:     "replace username placeholder",
			pattern:  "user/${username}/data",
			username: "alice",
			clientID: "device1",
			want:     "user/alice/data",
		},
		{
			name:     "replace clientid placeholder",
			pattern:  "device/${clientid}/telemetry",
			username: "alice",
			clientID: "sensor-001",
			want:     "device/sensor-001/telemetry",
		},
		{
			name:     "replace both placeholders",
			pattern:  "users/${username}/devices/${clientid}/status",
			username: "bob",
			clientID: "device-123",
			want:     "users/bob/devices/device-123/status",
		},
		{
			name:     "no placeholders",
			pattern:  "static/topic/path",
			username: "alice",
			clientID: "device1",
			want:     "static/topic/path",
		},
		{
			name:     "multiple username placeholders",
			pattern:  "${username}/${username}/data",
			username: "charlie",
			clientID: "device1",
			want:     "charlie/charlie/data",
		},
		{
			name:     "with wildcards and placeholders",
			pattern:  "user/${username}/+/${clientid}/#",
			username: "dave",
			clientID: "dev-999",
			want:     "user/dave/+/dev-999/#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replacePlaceholders(tt.pattern, tt.username, tt.clientID)
			if got != tt.want {
				t.Errorf("replacePlaceholders(%q, %q, %q) = %q, want %q",
					tt.pattern, tt.username, tt.clientID, got, tt.want)
			}
		})
	}
}

func TestCheckACLWithPlaceholders(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test users
	alice := createTestMQTTUser(t, db, "alice", "password123", "Alice")
	bob := createTestMQTTUser(t, db, "bob", "password123", "Bob")

	// Create ACL rules with placeholders
	createTestACLRule(t, db, alice.ID, "user/${username}/data", "pubsub")
	createTestACLRule(t, db, alice.ID, "device/${clientid}/telemetry", "pub")
	createTestACLRule(t, db, alice.ID, "users/${username}/devices/${clientid}/#", "pubsub")
	createTestACLRule(t, db, bob.ID, "user/${username}/#", "pubsub")

	tests := []struct {
		name        string
		username    string
		clientID    string
		topic       string
		action      string
		wantAllowed bool
	}{
		// Username placeholder tests
		{
			name:        "alice can publish to own user data topic",
			username:    "alice",
			clientID:    "device1",
			topic:       "user/alice/data",
			action:      "pub",
			wantAllowed: true,
		},
		{
			name:        "alice cannot publish to bob's user data topic",
			username:    "alice",
			clientID:    "device1",
			topic:       "user/bob/data",
			action:      "pub",
			wantAllowed: false,
		},
		{
			name:        "bob can publish to any subtopic under their user namespace",
			username:    "bob",
			clientID:    "device1",
			topic:       "user/bob/data/sensor/temp",
			action:      "pub",
			wantAllowed: true,
		},

		// ClientID placeholder tests
		{
			name:        "alice can publish telemetry from device matching clientID",
			username:    "alice",
			clientID:    "sensor-001",
			topic:       "device/sensor-001/telemetry",
			action:      "pub",
			wantAllowed: true,
		},
		{
			name:        "alice cannot publish telemetry from device not matching clientID",
			username:    "alice",
			clientID:    "sensor-001",
			topic:       "device/sensor-002/telemetry",
			action:      "pub",
			wantAllowed: false,
		},

		// Combined placeholders
		{
			name:        "alice can access topic with both username and clientID",
			username:    "alice",
			clientID:    "device-123",
			topic:       "users/alice/devices/device-123/status",
			action:      "pub",
			wantAllowed: true,
		},
		{
			name:        "alice can access nested topics with placeholders and wildcards",
			username:    "alice",
			clientID:    "device-456",
			topic:       "users/alice/devices/device-456/sensors/temp",
			action:      "sub",
			wantAllowed: true,
		},
		{
			name:        "alice cannot access topic with wrong username",
			username:    "alice",
			clientID:    "device-123",
			topic:       "users/bob/devices/device-123/status",
			action:      "pub",
			wantAllowed: false,
		},
		{
			name:        "alice cannot access topic with wrong clientID",
			username:    "alice",
			clientID:    "device-123",
			topic:       "users/alice/devices/device-999/status",
			action:      "pub",
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := db.CheckACL(tt.username, tt.clientID, tt.topic, tt.action)
			if err != nil {
				t.Fatalf("CheckACL() unexpected error: %v", err)
			}

			if allowed != tt.wantAllowed {
				t.Errorf("CheckACL() allowed = %v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestCreateProvisionedACLRule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user := createTestMQTTUser(t, db, "testuser", "password123", "Test user")

	tests := []struct {
		name         string
		userID       uint
		topicPattern string
		permission   string
		wantErr      bool
	}{
		{
			name:         "create provisioned pub rule",
			userID:       user.ID,
			topicPattern: "test/pub/#",
			permission:   "pub",
			wantErr:      false,
		},
		{
			name:         "create provisioned sub rule",
			userID:       user.ID,
			topicPattern: "test/sub/#",
			permission:   "sub",
			wantErr:      false,
		},
		{
			name:         "create provisioned pubsub rule",
			userID:       user.ID,
			topicPattern: "test/pubsub/#",
			permission:   "pubsub",
			wantErr:      false,
		},
		{
			name:         "invalid permission",
			userID:       user.ID,
			topicPattern: "test/#",
			permission:   "invalid",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.CreateProvisionedACLRule(tt.userID, tt.topicPattern, tt.permission)

			if tt.wantErr {
				if err == nil {
					t.Error("CreateProvisionedACLRule() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateProvisionedACLRule() unexpected error: %v", err)
			}

			// Verify the rule was created and marked as provisioned
			rules, err := db.GetACLRulesByMQTTUserID(int(tt.userID))
			if err != nil {
				t.Fatalf("GetACLRulesByMQTTUserID() failed: %v", err)
			}

			found := false
			for _, rule := range rules {
				if rule.TopicPattern == tt.topicPattern && rule.Permission == tt.permission {
					found = true
					if !rule.ProvisionedFromConfig {
						t.Error("rule should be marked as provisioned")
					}
					break
				}
			}

			if !found {
				t.Errorf("rule with pattern '%s' not found", tt.topicPattern)
			}
		})
	}
}

func TestDeleteProvisionedACLRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user := createTestMQTTUser(t, db, "testuser", "password123", "Test user")

	// Create both provisioned and manual rules
	db.CreateProvisionedACLRule(user.ID, "provisioned/1/#", "pub")
	db.CreateProvisionedACLRule(user.ID, "provisioned/2/#", "sub")
	db.CreateACLRule(int(user.ID), "manual/1/#", "pubsub")

	// Verify all rules exist
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("GetACLRulesByMQTTUserID() failed: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules initially, got %d", len(rules))
	}

	// Delete only provisioned rules
	err = db.DeleteProvisionedACLRules(user.ID)
	if err != nil {
		t.Fatalf("DeleteProvisionedACLRules() unexpected error: %v", err)
	}

	// Verify only manual rule remains
	rules, err = db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("GetACLRulesByMQTTUserID() failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule after deletion, got %d", len(rules))
	}

	// Verify the remaining rule is the manual one
	if rules[0].TopicPattern != "manual/1/#" {
		t.Errorf("expected manual rule to remain, got '%s'", rules[0].TopicPattern)
	}
	if rules[0].ProvisionedFromConfig {
		t.Error("remaining rule should not be marked as provisioned")
	}
}

func TestDeleteProvisionedACLRules_MultipleUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user1 := createTestMQTTUser(t, db, "user1", "pass1", "User 1")
	user2 := createTestMQTTUser(t, db, "user2", "pass2", "User 2")

	// Create provisioned rules for both users
	db.CreateProvisionedACLRule(user1.ID, "user1/#", "pubsub")
	db.CreateProvisionedACLRule(user2.ID, "user2/#", "pubsub")

	// Delete provisioned rules for user1 only
	err := db.DeleteProvisionedACLRules(user1.ID)
	if err != nil {
		t.Fatalf("DeleteProvisionedACLRules() unexpected error: %v", err)
	}

	// Verify user1's rules are deleted
	rules1, _ := db.GetACLRulesByMQTTUserID(int(user1.ID))
	if len(rules1) != 0 {
		t.Errorf("expected 0 rules for user1, got %d", len(rules1))
	}

	// Verify user2's rules are untouched
	rules2, _ := db.GetACLRulesByMQTTUserID(int(user2.ID))
	if len(rules2) != 1 {
		t.Errorf("expected 1 rule for user2, got %d", len(rules2))
	}
}
