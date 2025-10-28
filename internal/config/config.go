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
	Bridges  []BridgeConfig   `yaml:"bridges"`
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

// BridgeConfig represents an MQTT bridge in the config file
type BridgeConfig struct {
	Name              string                 `yaml:"name"`
	RemoteHost        string                 `yaml:"remote_host"`
	RemotePort        int                    `yaml:"remote_port,omitempty"`
	RemoteUsername    string                 `yaml:"remote_username,omitempty"`
	RemotePassword    string                 `yaml:"remote_password,omitempty"`
	ClientID          string                 `yaml:"client_id,omitempty"`
	CleanSession      bool                   `yaml:"clean_session,omitempty"`
	KeepAlive         int                    `yaml:"keep_alive,omitempty"`
	ConnectionTimeout int                    `yaml:"connection_timeout,omitempty"`
	Metadata          map[string]interface{} `yaml:"metadata,omitempty"`
	Topics            []BridgeTopicConfig    `yaml:"topics"`
}

// BridgeTopicConfig represents a topic mapping in a bridge configuration
type BridgeTopicConfig struct {
	LocalPattern  string `yaml:"local_pattern"`
	RemotePattern string `yaml:"remote_pattern"`
	Direction     string `yaml:"direction"`
	QoS           int    `yaml:"qos,omitempty"`
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

	// Validate bridges
	bridgeNames := make(map[string]bool)
	for _, bridge := range c.Bridges {
		if bridge.Name == "" {
			return fmt.Errorf("bridge missing name")
		}
		if bridge.RemoteHost == "" {
			return fmt.Errorf("bridge '%s' missing remote_host", bridge.Name)
		}
		if bridgeNames[bridge.Name] {
			return fmt.Errorf("duplicate bridge name: %s", bridge.Name)
		}
		bridgeNames[bridge.Name] = true

		// Set defaults
		if bridge.RemotePort == 0 {
			bridge.RemotePort = 1883
		}
		if bridge.RemotePort < 1 || bridge.RemotePort > 65535 {
			return fmt.Errorf("bridge '%s' has invalid remote_port: %d", bridge.Name, bridge.RemotePort)
		}

		// Validate topics
		if len(bridge.Topics) == 0 {
			return fmt.Errorf("bridge '%s' has no topics configured", bridge.Name)
		}
		for _, topic := range bridge.Topics {
			if topic.LocalPattern == "" {
				return fmt.Errorf("bridge '%s' has topic with empty local_pattern", bridge.Name)
			}
			if topic.RemotePattern == "" {
				return fmt.Errorf("bridge '%s' has topic with empty remote_pattern", bridge.Name)
			}
			if topic.Direction != "in" && topic.Direction != "out" && topic.Direction != "both" {
				return fmt.Errorf("bridge '%s' has invalid direction '%s' (must be in, out, or both)", bridge.Name, topic.Direction)
			}
			if topic.QoS < 0 || topic.QoS > 2 {
				return fmt.Errorf("bridge '%s' has invalid QoS %d (must be 0, 1, or 2)", bridge.Name, topic.QoS)
			}
		}
	}

	return nil
}
