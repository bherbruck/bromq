package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the MQTT server provisioning configuration
type Config struct {
	Users    []MQTTUserConfig `yaml:"users"`
	ACLRules []ACLRuleConfig  `yaml:"acl_rules"`
}

// MQTTUserConfig represents an MQTT user in the config file
type MQTTUserConfig struct {
	Username    string                 `yaml:"username"`
	Password    string                 `yaml:"password"`
	Description string                 `yaml:"description,omitempty"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty"`
}

// ACLRuleConfig represents an ACL rule in the config file
type ACLRuleConfig struct {
	MQTTUsername string `yaml:"mqtt_username"`
	TopicPattern string `yaml:"topic_pattern"`
	Permission   string `yaml:"permission"`
}

// Load reads and parses a YAML config file with environment variable interpolation
func Load(path string) (*Config, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Protect reserved placeholders before env var expansion
	// Replace ${username} and ${clientid} with temporary markers
	content := string(data)
	content = strings.ReplaceAll(content, "${username}", "__RESERVED_USERNAME__")
	content = strings.ReplaceAll(content, "${clientid}", "__RESERVED_CLIENTID__")

	// Expand environment variables (will not touch our markers)
	expanded := os.ExpandEnv(content)

	// Restore reserved placeholders
	expanded = strings.ReplaceAll(expanded, "__RESERVED_USERNAME__", "${username}")
	expanded = strings.ReplaceAll(expanded, "__RESERVED_CLIENTID__", "${clientid}")

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	// Check for duplicate usernames
	seen := make(map[string]bool)
	for _, user := range c.Users {
		if user.Username == "" {
			return fmt.Errorf("user missing username")
		}
		if user.Password == "" {
			return fmt.Errorf("user '%s' missing password", user.Username)
		}
		if seen[user.Username] {
			return fmt.Errorf("duplicate username: %s", user.Username)
		}
		seen[user.Username] = true
	}

	// Validate ACL rules
	validUsernames := make(map[string]bool)
	for _, user := range c.Users {
		validUsernames[user.Username] = true
	}

	for _, rule := range c.ACLRules {
		if rule.MQTTUsername == "" {
			return fmt.Errorf("ACL rule missing mqtt_username")
		}
		if rule.TopicPattern == "" {
			return fmt.Errorf("ACL rule for user '%s' missing topic_pattern", rule.MQTTUsername)
		}
		if rule.Permission == "" {
			return fmt.Errorf("ACL rule for user '%s' missing permission", rule.MQTTUsername)
		}

		// Check if username exists
		if !validUsernames[rule.MQTTUsername] {
			return fmt.Errorf("ACL rule references unknown user: %s", rule.MQTTUsername)
		}

		// Validate permission
		if rule.Permission != "pub" && rule.Permission != "sub" && rule.Permission != "pubsub" {
			return fmt.Errorf("ACL rule for user '%s' has invalid permission: %s (must be pub, sub, or pubsub)", rule.MQTTUsername, rule.Permission)
		}
	}

	return nil
}
