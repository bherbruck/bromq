package provisioning

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github/bherbruck/bromq/internal/config"
	"github/bherbruck/bromq/internal/storage"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *storage.DB {
	cfg := storage.DefaultSQLiteConfig(":memory:")
	// Use isolated Prometheus registry to prevent duplicate registration in tests
	cache := storage.NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := storage.OpenWithCache(cfg, cache)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db
}

func TestProvision_NewUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{
				Username:    "test_user",
				Password:    "password123",
				Description: "Test user",
			},
		},
		ACLRules: []config.ACLRuleConfig{
			{
				MQTTUsername: "test_user",
				TopicPattern: "test/#",
				Permission:   "pubsub",
			},
		},
	}

	// Provision
	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify user was created
	user, err := db.GetMQTTUserByUsername("test_user")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user.Username != "test_user" {
		t.Errorf("expected username 'test_user', got '%s'", user.Username)
	}
	if user.Description != "Test user" {
		t.Errorf("expected description 'Test user', got '%s'", user.Description)
	}
	if !user.ProvisionedFromConfig {
		t.Error("expected user to be marked as provisioned")
	}

	// Verify ACL rule was created
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("failed to get ACL rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 ACL rule, got %d", len(rules))
	}
	if rules[0].TopicPattern != "test/#" {
		t.Errorf("expected topic pattern 'test/#', got '%s'", rules[0].TopicPattern)
	}
	if !rules[0].ProvisionedFromConfig {
		t.Error("expected ACL rule to be marked as provisioned")
	}
}

func TestProvision_UpdateExistingUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// First provision
	cfg1 := &config.Config{
		Users: []config.MQTTUserConfig{
			{
				Username:    "test_user",
				Password:    "password123",
				Description: "Original description",
			},
		},
		ACLRules: []config.ACLRuleConfig{
			{
				MQTTUsername: "test_user",
				TopicPattern: "test/#",
				Permission:   "pub",
			},
		},
	}

	if err := Provision(db, cfg1); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	// Get original user ID
	user1, _ := db.GetMQTTUserByUsername("test_user")
	originalID := user1.ID

	// Second provision with updated config
	cfg2 := &config.Config{
		Users: []config.MQTTUserConfig{
			{
				Username:    "test_user",
				Password:    "newpassword456",
				Description: "Updated description",
			},
		},
		ACLRules: []config.ACLRuleConfig{
			{
				MQTTUsername: "test_user",
				TopicPattern: "updated/#",
				Permission:   "pubsub",
			},
		},
	}

	if err := Provision(db, cfg2); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Verify user was updated (same ID)
	user2, err := db.GetMQTTUserByUsername("test_user")
	if err != nil {
		t.Fatalf("failed to get updated user: %v", err)
	}
	if user2.ID != originalID {
		t.Errorf("user ID changed from %d to %d", originalID, user2.ID)
	}
	if user2.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got '%s'", user2.Description)
	}

	// Verify password was updated
	if _, err := db.AuthenticateMQTTUser("test_user", "newpassword456"); err != nil {
		t.Error("expected new password to work")
	}
	if _, err := db.AuthenticateMQTTUser("test_user", "password123"); err == nil {
		t.Error("expected old password to fail")
	}

	// Verify ACL rules were replaced
	rules, err := db.GetACLRulesByMQTTUserID(int(user2.ID))
	if err != nil {
		t.Fatalf("failed to get ACL rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 ACL rule, got %d", len(rules))
	}
	if rules[0].TopicPattern != "updated/#" {
		t.Errorf("expected topic pattern 'updated/#', got '%s'", rules[0].TopicPattern)
	}
	if rules[0].Permission != "pubsub" {
		t.Errorf("expected permission 'pubsub', got '%s'", rules[0].Permission)
	}
}

func TestProvision_RemoveOrphanedUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// First provision with two users
	cfg1 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "user1", Password: "pass1"},
			{Username: "user2", Password: "pass2"},
		},
		ACLRules: []config.ACLRuleConfig{},
	}

	if err := Provision(db, cfg1); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	// Verify both users exist
	if _, err := db.GetMQTTUserByUsername("user1"); err != nil {
		t.Error("user1 should exist")
	}
	if _, err := db.GetMQTTUserByUsername("user2"); err != nil {
		t.Error("user2 should exist")
	}

	// Second provision with only one user
	cfg2 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "user1", Password: "pass1"},
		},
		ACLRules: []config.ACLRuleConfig{},
	}

	if err := Provision(db, cfg2); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Verify user1 still exists
	if _, err := db.GetMQTTUserByUsername("user1"); err != nil {
		t.Error("user1 should still exist")
	}

	// Verify user2 was removed
	if _, err := db.GetMQTTUserByUsername("user2"); err == nil {
		t.Error("user2 should have been removed")
	}
}

func TestProvision_ManualUsersNotTouched(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a manual user (not provisioned)
	manualUser, err := db.CreateMQTTUser("manual_user", "manual_pass", "Manual user", nil)
	if err != nil {
		t.Fatalf("failed to create manual user: %v", err)
	}

	// Create manual ACL rule
	_, err = db.CreateACLRule(int(manualUser.ID), "manual/#", "pubsub")
	if err != nil {
		t.Fatalf("failed to create manual ACL rule: %v", err)
	}

	// Provision a different user
	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "provisioned_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{
				MQTTUsername: "provisioned_user",
				TopicPattern: "provisioned/#",
				Permission:   "pub",
			},
		},
	}

	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify manual user still exists
	user, err := db.GetMQTTUserByUsername("manual_user")
	if err != nil {
		t.Error("manual user should still exist")
	}
	if user.ProvisionedFromConfig {
		t.Error("manual user should not be marked as provisioned")
	}

	// Verify manual ACL rule still exists
	rules, err := db.GetACLRulesByMQTTUserID(int(manualUser.ID))
	if err != nil {
		t.Fatalf("failed to get manual ACL rules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 manual ACL rule, got %d", len(rules))
	}
	if rules[0].ProvisionedFromConfig {
		t.Error("manual ACL rule should not be marked as provisioned")
	}
}

func TestProvision_MultipleACLRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "user1", Password: "pass1"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "user1", TopicPattern: "sensors/#", Permission: "pub"},
			{MQTTUsername: "user1", TopicPattern: "commands/#", Permission: "sub"},
			{MQTTUsername: "user1", TopicPattern: "status/+", Permission: "pubsub"},
		},
	}

	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify all ACL rules were created
	user, _ := db.GetMQTTUserByUsername("user1")
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("failed to get ACL rules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 ACL rules, got %d", len(rules))
	}

	// Verify all are marked as provisioned
	for i, rule := range rules {
		if !rule.ProvisionedFromConfig {
			t.Errorf("ACL rule %d should be marked as provisioned", i)
		}
	}
}

func TestProvision_WithMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{
				Username:    "sensor",
				Password:    "pass123",
				Description: "Sensor device",
				Metadata: map[string]interface{}{
					"location":    "warehouse",
					"device_type": "temperature",
					"max_rate":    100,
				},
			},
		},
		ACLRules: []config.ACLRuleConfig{},
	}

	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify metadata was stored
	user, err := db.GetMQTTUserByUsername("sensor")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}
	// Note: We can't easily test the JSON content without unmarshaling,
	// but the fact that it was stored is a good indicator
}

func TestProvision_EmptyConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.Config{
		Users:    []config.MQTTUserConfig{},
		ACLRules: []config.ACLRuleConfig{},
	}

	// Should not error on empty config
	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Provision with empty config failed: %v", err)
	}
}

func TestProvision_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test", TopicPattern: "test/#", Permission: "pubsub"},
		},
	}

	// Provision multiple times
	for i := 0; i < 3; i++ {
		if err := Provision(db, cfg); err != nil {
			t.Fatalf("Provision iteration %d failed: %v", i, err)
		}
	}

	// Should only have one MQTT user (admin is a DashboardUser, not an MQTTUser)
	users, err := db.ListMQTTUsers()
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}
	if len(users) != 1 { // just test user
		t.Errorf("expected 1 MQTT user (test), got %d", len(users))
	}

	// Should only have one ACL rule
	user, _ := db.GetMQTTUserByUsername("test")
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("failed to get ACL rules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 ACL rule, got %d", len(rules))
	}
}

func TestProvision_RemoveAllACLRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// First provision with ACL rules
	cfg1 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"},
			{MQTTUsername: "test_user", TopicPattern: "data/#", Permission: "sub"},
		},
	}

	if err := Provision(db, cfg1); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	// Verify ACL rules were created
	user, _ := db.GetMQTTUserByUsername("test_user")
	rules, err := db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("failed to get ACL rules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 ACL rules after first provision, got %d", len(rules))
	}

	// Second provision with NO ACL rules for the user
	cfg2 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{}, // No ACL rules anymore
	}

	if err := Provision(db, cfg2); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Verify all ACL rules were removed
	rules, err = db.GetACLRulesByMQTTUserID(int(user.ID))
	if err != nil {
		t.Fatalf("failed to get ACL rules after removal: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 ACL rules after removing all from config, got %d", len(rules))
		for i, rule := range rules {
			t.Logf("  Rule %d: %s (%s)", i, rule.TopicPattern, rule.Permission)
		}
	}

	// User should still exist
	user2, err := db.GetMQTTUserByUsername("test_user")
	if err != nil {
		t.Error("user should still exist after removing ACL rules")
	}
	if user2.ID != user.ID {
		t.Error("user ID should not change")
	}
}

func TestProvision_ACLRuleIDsPreservedWhenUnchanged(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"},
			{MQTTUsername: "test_user", TopicPattern: "data/#", Permission: "sub"},
		},
	}

	// First provision
	if err := Provision(db, cfg); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	// Get initial rule IDs
	user, _ := db.GetMQTTUserByUsername("test_user")
	rules1, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules1) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules1))
	}

	// Second provision with same config
	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Get rules again and verify IDs didn't change
	rules2, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules2) != 2 {
		t.Fatalf("expected 2 rules after second provision, got %d", len(rules2))
	}

	// IDs should be preserved (no delete/recreate happened)
	ruleIDs1 := make(map[uint]bool)
	for _, rule := range rules1 {
		ruleIDs1[rule.ID] = true
	}

	for _, rule := range rules2 {
		if !ruleIDs1[rule.ID] {
			t.Errorf("Rule ID %d is new - rule was recreated instead of preserved", rule.ID)
		}
	}
}

func TestProvision_AddNewACLRule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Initial config with one rule
	cfg1 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"},
		},
	}

	if err := Provision(db, cfg1); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	user, _ := db.GetMQTTUserByUsername("test_user")
	rules1, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules1) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules1))
	}
	oldRuleID := rules1[0].ID

	// Add a new rule
	cfg2 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"},
			{MQTTUsername: "test_user", TopicPattern: "data/#", Permission: "sub"}, // NEW
		},
	}

	if err := Provision(db, cfg2); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Should now have 2 rules
	rules2, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules2) != 2 {
		t.Fatalf("expected 2 rules after adding, got %d", len(rules2))
	}

	// Old rule should still have same ID (not recreated)
	foundOld := false
	foundNew := false
	for _, rule := range rules2 {
		if rule.TopicPattern == "test/#" && rule.Permission == "pub" {
			foundOld = true
			if rule.ID != oldRuleID {
				t.Errorf("Old rule ID changed from %d to %d (should be preserved)", oldRuleID, rule.ID)
			}
		}
		if rule.TopicPattern == "data/#" && rule.Permission == "sub" {
			foundNew = true
		}
	}

	if !foundOld {
		t.Error("Old rule not found")
	}
	if !foundNew {
		t.Error("New rule not found")
	}
}

func TestProvision_RemoveOneACLRule(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Initial config with two rules
	cfg1 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"},
			{MQTTUsername: "test_user", TopicPattern: "data/#", Permission: "sub"},
		},
	}

	if err := Provision(db, cfg1); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	user, _ := db.GetMQTTUserByUsername("test_user")
	rules1, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules1) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules1))
	}

	// Find the rule we're keeping
	var keepRuleID uint
	for _, rule := range rules1 {
		if rule.TopicPattern == "test/#" {
			keepRuleID = rule.ID
		}
	}

	// Remove one rule
	cfg2 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"}, // Keep this one
			// data/# removed
		},
	}

	if err := Provision(db, cfg2); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Should now have 1 rule
	rules2, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules2) != 1 {
		t.Fatalf("expected 1 rule after removing, got %d", len(rules2))
	}

	// Verify it's the right rule and ID is preserved
	if rules2[0].TopicPattern != "test/#" {
		t.Errorf("Wrong rule kept: %s", rules2[0].TopicPattern)
	}
	if rules2[0].ID != keepRuleID {
		t.Errorf("Rule ID changed from %d to %d (should be preserved)", keepRuleID, rules2[0].ID)
	}
}

func TestProvision_MixedACLChanges(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Initial config
	cfg1 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "test/#", Permission: "pub"},
			{MQTTUsername: "test_user", TopicPattern: "data/#", Permission: "sub"},
			{MQTTUsername: "test_user", TopicPattern: "temp/#", Permission: "pubsub"},
		},
	}

	if err := Provision(db, cfg1); err != nil {
		t.Fatalf("First provision failed: %v", err)
	}

	user, _ := db.GetMQTTUserByUsername("test_user")
	rules1, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules1) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules1))
	}

	// Find ID of rule we're keeping
	var keepRuleID uint
	for _, rule := range rules1 {
		if rule.TopicPattern == "data/#" {
			keepRuleID = rule.ID
		}
	}

	// Mixed changes: remove test/#, keep data/#, remove temp/#, add new/#
	cfg2 := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "data/#", Permission: "sub"},   // KEEP
			{MQTTUsername: "test_user", TopicPattern: "new/#", Permission: "pubsub"}, // ADD
			// test/# REMOVED
			// temp/# REMOVED
		},
	}

	if err := Provision(db, cfg2); err != nil {
		t.Fatalf("Second provision failed: %v", err)
	}

	// Should now have 2 rules
	rules2, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules2) != 2 {
		t.Fatalf("expected 2 rules after mixed changes, got %d", len(rules2))
	}

	// Verify kept rule has same ID
	foundKept := false
	foundNew := false
	for _, rule := range rules2 {
		if rule.TopicPattern == "data/#" {
			foundKept = true
			if rule.ID != keepRuleID {
				t.Errorf("Kept rule ID changed from %d to %d", keepRuleID, rule.ID)
			}
		}
		if rule.TopicPattern == "new/#" {
			foundNew = true
		}
	}

	if !foundKept {
		t.Error("Kept rule not found")
	}
	if !foundNew {
		t.Error("New rule not found")
	}
}

func TestProvision_ManualACLRulesNotTouched(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create user and manual rule
	user, _ := db.CreateMQTTUser("test_user", "pass123", "", nil)
	manualRule, _ := db.CreateACLRule(int(user.ID), "manual/#", "pub")

	// Provision with different rules
	cfg := &config.Config{
		Users: []config.MQTTUserConfig{
			{Username: "test_user", Password: "pass123"},
		},
		ACLRules: []config.ACLRuleConfig{
			{MQTTUsername: "test_user", TopicPattern: "provisioned/#", Permission: "sub"},
		},
	}

	if err := Provision(db, cfg); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Should have both manual and provisioned rules
	rules, _ := db.GetACLRulesByMQTTUserID(int(user.ID))
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (1 manual + 1 provisioned), got %d", len(rules))
	}

	// Verify manual rule still exists with same ID
	foundManual := false
	for _, rule := range rules {
		if rule.TopicPattern == "manual/#" {
			foundManual = true
			if rule.ID != manualRule.ID {
				t.Error("Manual rule ID changed")
			}
			if rule.ProvisionedFromConfig {
				t.Error("Manual rule incorrectly marked as provisioned")
			}
		}
	}

	if !foundManual {
		t.Error("Manual rule was deleted (should be preserved)")
	}
}
