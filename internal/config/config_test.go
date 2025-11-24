package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		envVars     map[string]string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *Config)
	}{
		{
			name: "valid config with env vars",
			configYAML: `
users:
  - username: test_user
    password: ${TEST_PASSWORD}
    description: "Test user"
    metadata:
      location: "warehouse"

acl_rules:
  - username: test_user
    topic: "test/#"
    permission: pubsub
`,
			envVars: map[string]string{
				"TEST_PASSWORD": "secret123",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Users) != 1 {
					t.Errorf("expected 1 user, got %d", len(cfg.Users))
				}
				if cfg.Users[0].Username != "test_user" {
					t.Errorf("expected username 'test_user', got '%s'", cfg.Users[0].Username)
				}
				if cfg.Users[0].Password != "secret123" {
					t.Errorf("expected password 'secret123', got '%s'", cfg.Users[0].Password)
				}
				if len(cfg.ACLRules) != 1 {
					t.Errorf("expected 1 ACL rule, got %d", len(cfg.ACLRules))
				}
			},
		},
		{
			name: "valid config with metadata",
			configYAML: `
users:
  - username: sensor
    password: pass123
    description: "Sensor device"
    metadata:
      location: "warehouse"
      device_type: "temperature"
      max_connections: 10

acl_rules:
  - username: sensor
    topic: "sensors/${username}/#"
    permission: pub
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Users[0].Metadata["location"] != "warehouse" {
					t.Errorf("expected location 'warehouse', got '%v'", cfg.Users[0].Metadata["location"])
				}
				if cfg.Users[0].Metadata["device_type"] != "temperature" {
					t.Errorf("expected device_type 'temperature', got '%v'", cfg.Users[0].Metadata["device_type"])
				}
			},
		},
		{
			name: "reserved placeholders preserved in topic patterns",
			configYAML: `
users:
  - username: testuser
    password: ${TEST_PASS}
    description: "Test with reserved placeholders"

acl_rules:
  - username: testuser
    topic: "user/${username}/data"
    permission: pub
  - username: testuser
    topic: "device/${clientid}/status"
    permission: sub
  - username: testuser
    topic: "users/${username}/devices/${clientid}/#"
    permission: pubsub
`,
			envVars: map[string]string{
				"TEST_PASS": "mypassword",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Users[0].Password != "mypassword" {
					t.Errorf("expected password 'mypassword', got '%s'", cfg.Users[0].Password)
				}
				if len(cfg.ACLRules) != 3 {
					t.Fatalf("expected 3 ACL rules, got %d", len(cfg.ACLRules))
				}
				// Verify reserved placeholders are preserved
				expectedPatterns := []string{
					"user/${username}/data",
					"device/${clientid}/status",
					"users/${username}/devices/${clientid}/#",
				}
				for i, expected := range expectedPatterns {
					if cfg.ACLRules[i].Topic != expected {
						t.Errorf("rule %d: expected pattern '%s', got '%s'", i, expected, cfg.ACLRules[i].Topic)
					}
				}
			},
		},
		{
			name: "duplicate usernames",
			configYAML: `
users:
  - username: test_user
    password: pass1
  - username: test_user
    password: pass2

acl_rules: []
`,
			wantErr:     true,
			errContains: "duplicate username",
		},
		{
			name: "missing username",
			configYAML: `
users:
  - password: pass123

acl_rules: []
`,
			wantErr:     true,
			errContains: "missing username",
		},
		{
			name: "missing password",
			configYAML: `
users:
  - username: test_user

acl_rules: []
`,
			wantErr:     true,
			errContains: "missing password",
		},
		{
			name: "ACL references unknown user",
			configYAML: `
users:
  - username: user1
    password: pass123

acl_rules:
  - username: unknown_user
    topic: "test/#"
    permission: pubsub
`,
			wantErr:     true,
			errContains: "unknown user",
		},
		{
			name: "invalid permission",
			configYAML: `
users:
  - username: test_user
    password: pass123

acl_rules:
  - username: test_user
    topic: "test/#"
    permission: invalid
`,
			wantErr:     true,
			errContains: "invalid permission",
		},
		{
			name: "missing ACL topic pattern",
			configYAML: `
users:
  - username: test_user
    password: pass123

acl_rules:
  - username: test_user
    permission: pubsub
`,
			wantErr:     true,
			errContains: "missing topic",
		},
		{
			name: "multiple users and rules",
			configYAML: `
users:
  - username: user1
    password: ${PASS1}
    description: "User 1"
  - username: user2
    password: ${PASS2}
    description: "User 2"

acl_rules:
  - username: user1
    topic: "user1/#"
    permission: pubsub
  - username: user1
    topic: "shared/+"
    permission: sub
  - username: user2
    topic: "user2/#"
    permission: pub
`,
			envVars: map[string]string{
				"PASS1": "secret1",
				"PASS2": "secret2",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Users) != 2 {
					t.Errorf("expected 2 users, got %d", len(cfg.Users))
				}
				if len(cfg.ACLRules) != 3 {
					t.Errorf("expected 3 ACL rules, got %d", len(cfg.ACLRules))
				}
			},
		},
		{
			name: "env var with $VAR syntax",
			configYAML: `
users:
  - username: test
    password: $PASSWORD

acl_rules: []
`,
			envVars: map[string]string{
				"PASSWORD": "test123",
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Users[0].Password != "test123" {
					t.Errorf("expected password 'test123', got '%s'", cfg.Users[0].Password)
				}
			},
		},
		{
			name: "valid script with script_file",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: test-script
    description: "Test script"
    enabled: true
    file: /path/to/script.js
    triggers:
      - type: on_publish
        topic: "test/#"
        priority: 100
        enabled: true
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Scripts) != 1 {
					t.Fatalf("expected 1 script, got %d", len(cfg.Scripts))
				}
				if cfg.Scripts[0].Name != "test-script" {
					t.Errorf("expected name 'test-script', got '%s'", cfg.Scripts[0].Name)
				}
				if cfg.Scripts[0].File != "/path/to/script.js" {
					t.Errorf("expected script_file '/path/to/script.js', got '%s'", cfg.Scripts[0].File)
				}
				if len(cfg.Scripts[0].Triggers) != 1 {
					t.Errorf("expected 1 trigger, got %d", len(cfg.Scripts[0].Triggers))
				}
			},
		},
		{
			name: "valid script with content",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: inline-script
    description: "Inline script"
    enabled: true
    content: |
      log.info("Hello from inline script");
      mqtt.publish("output/topic", "hello", 1, false);
    triggers:
      - type: on_connect
        priority: 50
        enabled: true
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Scripts) != 1 {
					t.Fatalf("expected 1 script, got %d", len(cfg.Scripts))
				}
				if cfg.Scripts[0].Content == "" {
					t.Error("expected content to be set")
				}
				if cfg.Scripts[0].File != "" {
					t.Error("expected script_file to be empty")
				}
			},
		},
		{
			name: "script with both script_file and content (invalid)",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: invalid-script
    enabled: true
    file: /path/to/script.js
    content: "log.info('test');"
    triggers:
      - type: on_publish
        topic: "#"
        priority: 100
        enabled: true
`,
			wantErr:     true,
			errContains: "cannot have both file and content",
		},
		{
			name: "script with neither script_file nor content (invalid)",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: invalid-script
    enabled: true
    triggers:
      - type: on_publish
        topic: "#"
        priority: 100
        enabled: true
`,
			wantErr:     true,
			errContains: "must have either file or content",
		},
		{
			name: "script with invalid trigger type",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: test-script
    enabled: true
    content: "log.info('test');"
    triggers:
      - type: invalid_trigger
        topic: "#"
        priority: 100
        enabled: true
`,
			wantErr:     true,
			errContains: "invalid type",
		},
		{
			name: "script with missing trigger type",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: test-script
    enabled: true
    content: "log.info('test');"
    triggers:
      - topic: "#"
        priority: 100
        enabled: true
`,
			wantErr:     true,
			errContains: "missing type",
		},
		{
			name: "script with duplicate names",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: same-name
    enabled: true
    content: "log.info('1');"
    triggers:
      - type: on_publish
        topic: "#"
        priority: 100
        enabled: true
  - name: same-name
    enabled: true
    content: "log.info('2');"
    triggers:
      - type: on_publish
        topic: "#"
        priority: 100
        enabled: true
`,
			wantErr:     true,
			errContains: "duplicate script name",
		},
		{
			name: "script with metadata",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: metadata-script
    description: "Script with metadata"
    enabled: true
    content: "log.info('test');"
    metadata:
      environment: "production"
      max_retries: 3
    triggers:
      - type: on_publish
        topic: "sensors/#"
        priority: 75
        enabled: true
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Scripts) != 1 {
					t.Fatalf("expected 1 script, got %d", len(cfg.Scripts))
				}
				if cfg.Scripts[0].Metadata["environment"] != "production" {
					t.Errorf("expected environment 'production', got '%v'", cfg.Scripts[0].Metadata["environment"])
				}
				if cfg.Scripts[0].Metadata["max_retries"] != 3 {
					t.Errorf("expected max_retries 3, got '%v'", cfg.Scripts[0].Metadata["max_retries"])
				}
			},
		},
		{
			name: "script with multiple triggers",
			configYAML: `
users: []
acl_rules: []
scripts:
  - name: multi-trigger
    enabled: true
    content: "log.info('test');"
    triggers:
      - type: on_connect
        priority: 10
        enabled: true
      - type: on_disconnect
        priority: 10
        enabled: true
      - type: on_publish
        topic: "status/#"
        priority: 50
        enabled: true
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Scripts) != 1 {
					t.Fatalf("expected 1 script, got %d", len(cfg.Scripts))
				}
				if len(cfg.Scripts[0].Triggers) != 3 {
					t.Errorf("expected 3 triggers, got %d", len(cfg.Scripts[0].Triggers))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			// Load config
			cfg, err := Load(configPath)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Run validation function if provided
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			config: &Config{
				Users: []MQTTUserConfig{
					{Username: "user1", Password: "pass1"},
				},
				ACLRules: []ACLRuleConfig{
					{Username: "user1", Topic: "test/#", Permission: "pubsub"},
				},
			},
			wantErr: false,
		},
		{
			name: "all permission types",
			config: &Config{
				Users: []MQTTUserConfig{
					{Username: "user1", Password: "pass1"},
				},
				ACLRules: []ACLRuleConfig{
					{Username: "user1", Topic: "test/pub", Permission: "pub"},
					{Username: "user1", Topic: "test/sub", Permission: "sub"},
					{Username: "user1", Topic: "test/both", Permission: "pubsub"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty config",
			config: &Config{
				Users:    []MQTTUserConfig{},
				ACLRules: []ACLRuleConfig{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestJavaScriptTemplateLiteralsInScripts(t *testing.T) {
	configYAML := `
users:
  - username: testuser
    password: testpass

scripts:
  - name: test-template-literals
    enabled: true
    content: |
      const msg = "Hello $${world}";
      const topic = 'devices/$${deviceId}/status';
      mqtt.publish(topic, msg);
    triggers:
      - type: on_publish
        topic: "test/#"
        enabled: true
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify JavaScript template literals are preserved (after $$ unescaping)
	if len(cfg.Scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(cfg.Scripts))
	}

	script := cfg.Scripts[0].Content
	if !strings.Contains(script, "${world}") {
		t.Errorf("Expected ${world} after $$ unescaping, got: %s", script)
	}
	if !strings.Contains(script, "${deviceId}") {
		t.Errorf("Expected ${deviceId} after $$ unescaping, got: %s", script)
	}
}

func TestDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		config   string
		expected string // Expected value after expansion
	}{
		{
			name:    "default used when env var not set",
			envVars: map[string]string{},
			config: `users:
  - username: test
    password: ${MISSING_VAR:-default_password}
`,
			expected: "default_password",
		},
		{
			name:    "env var used when set (ignores default)",
			envVars: map[string]string{"PRESENT_VAR": "actual_value"},
			config: `users:
  - username: test
    password: ${PRESENT_VAR:-default_password}
`,
			expected: "actual_value",
		},
		{
			name:    "empty env var uses default",
			envVars: map[string]string{"EMPTY_VAR": ""},
			config: `users:
  - username: test
    password: ${EMPTY_VAR:-default_password}
`,
			expected: "default_password",
		},
		{
			name:    "default with special characters",
			envVars: map[string]string{},
			config: `users:
  - username: test
    password: ${MISSING:-p@ss:w0rd!}
`,
			expected: "p@ss:w0rd!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Write config
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yml")
			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			// Load and verify
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			if cfg.Users[0].Password != tt.expected {
				t.Errorf("expected password '%s', got '%s'", tt.expected, cfg.Users[0].Password)
			}
		})
	}
}

func TestDollarEscaping(t *testing.T) {
	configYAML := `
users:
  - username: testuser
    password: testpass

scripts:
  - name: test-escaping
    enabled: true
    content: |
      const literal = "$${notExpanded}";
      const template = 'topic/$${clientId}/data';
      log.info("Using $$LITERAL_DOLLAR signs");
    triggers:
      - type: on_publish
        topic: "test/#"
        enabled: true
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	script := cfg.Scripts[0].Content

	// $$ should become $ after processing
	if !strings.Contains(script, "${notExpanded}") {
		t.Errorf("Expected ${notExpanded} after $$ unescaping, got: %s", script)
	}
	if !strings.Contains(script, "${clientId}") {
		t.Errorf("Expected ${clientId} after $$ unescaping, got: %s", script)
	}
	if !strings.Contains(script, "$LITERAL_DOLLAR") {
		t.Errorf("Expected $LITERAL_DOLLAR after $$ unescaping, got: %s", script)
	}
}

func TestEnvVarsInScripts(t *testing.T) {
	os.Setenv("API_KEY", "secret123")
	os.Setenv("ENDPOINT", "https://api.example.com")
	defer os.Unsetenv("API_KEY")
	defer os.Unsetenv("ENDPOINT")

	configYAML := `
users:
  - username: testuser
    password: testpass

scripts:
  - name: test-env-vars
    enabled: true
    content: |
      const apiKey = "${API_KEY}";
      const endpoint = "${ENDPOINT}";
      const topic = 'device/$${deviceId}/status';
      log.info(apiKey, endpoint, topic);
    triggers:
      - type: on_publish
        topic: "test/#"
        enabled: true
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	script := cfg.Scripts[0].Content

	// Env vars should be expanded
	if !strings.Contains(script, "secret123") {
		t.Errorf("Expected API_KEY to be expanded to 'secret123', got: %s", script)
	}
	if !strings.Contains(script, "https://api.example.com") {
		t.Errorf("Expected ENDPOINT to be expanded, got: %s", script)
	}

	// Template literal should be unescaped
	if !strings.Contains(script, "${deviceId}") {
		t.Errorf("Expected ${deviceId} after $$ unescaping, got: %s", script)
	}
}

func TestReservedPlaceholdersStillWork(t *testing.T) {
	os.Setenv("username", "SHOULD_NOT_EXPAND") // Set conflicting env var
	os.Setenv("clientid", "SHOULD_NOT_EXPAND")
	defer os.Unsetenv("username")
	defer os.Unsetenv("clientid")

	configYAML := `
users:
  - username: testuser
    password: testpass

acl_rules:
  - username: testuser
    topic: "user/${username}/data"
    permission: pub
  - username: testuser
    topic: "device/${clientid}/#"
    permission: sub
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Reserved placeholders should NOT be expanded even if env vars exist
	if cfg.ACLRules[0].Topic != "user/${username}/data" {
		t.Errorf("Expected ${username} to be preserved, got: %s", cfg.ACLRules[0].Topic)
	}
	if cfg.ACLRules[1].Topic != "device/${clientid}/#" {
		t.Errorf("Expected ${clientid} to be preserved, got: %s", cfg.ACLRules[1].Topic)
	}
}
