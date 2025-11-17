package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the MQTT server provisioning configuration
type Config struct {
	Users    []MQTTUserConfig `yaml:"users" json:"users,omitempty" jsonschema:"title=MQTT Users,description=MQTT authentication credentials for devices (not dashboard users)"`
	ACLRules []ACLRuleConfig  `yaml:"acl_rules" json:"acl_rules,omitempty" jsonschema:"title=ACL Rules,description=Access control rules for MQTT topic permissions"`
	Bridges  []BridgeConfig   `yaml:"bridges" json:"bridges,omitempty" jsonschema:"title=MQTT Bridges,description=Bridge connections to remote MQTT brokers for message forwarding"`
	Scripts  []ScriptConfig   `yaml:"scripts" json:"scripts,omitempty" jsonschema:"title=JavaScript Scripts,description=Custom JavaScript scripts that execute on MQTT events"`
}

// MQTTUserConfig represents an MQTT user in the config file
type MQTTUserConfig struct {
	Username    string                 `yaml:"username" json:"username" jsonschema:"required,title=Username,description=MQTT username for device authentication. Supports env vars: ${VAR} or ${VAR:-default},minLength=1,example=sensor_user"`
	Password    string                 `yaml:"password" json:"password" jsonschema:"required,title=Password,description=MQTT password. Supports env vars: ${PASSWORD} or ${PASSWORD:-default},minLength=1,example=${SENSOR_PASSWORD}"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"title=Description,description=Human-readable description of this MQTT user,example=Temperature and humidity sensors"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty" jsonschema:"title=Metadata,description=Custom metadata key-value pairs (any valid JSON)"`
}

// ACLRuleConfig represents an ACL rule in the config file
type ACLRuleConfig struct {
	Username   string `yaml:"username" json:"username" jsonschema:"required,title=Username,description=MQTT username this rule applies to (must exist in users list),minLength=1,example=sensor_user"`
	Topic      string `yaml:"topic" json:"topic" jsonschema:"required,title=Topic Pattern,description=MQTT topic pattern with wildcards (+/#) and runtime placeholders (${username}/${clientid}),minLength=1,example=sensors/${username}/#"`
	Permission string `yaml:"permission" json:"permission" jsonschema:"required,title=Permission,description=Access permission for this topic pattern,enum=pub,enum=sub,enum=pubsub"`
}

// BridgeConfig represents an MQTT bridge in the config file
type BridgeConfig struct {
	Name              string                 `yaml:"name" json:"name" jsonschema:"required,title=Bridge Name,description=Unique name for this bridge connection,minLength=1,example=cloud-bridge"`
	Host              string                 `yaml:"host" json:"host" jsonschema:"required,title=Remote Host,description=Remote MQTT broker hostname or IP. Supports env vars: ${HOST:-default},minLength=1,example=${CLOUD_MQTT_HOST:-mqtt.example.com}"`
	Port              int                    `yaml:"port,omitempty" json:"port,omitempty" jsonschema:"title=Remote Port,description=Remote MQTT broker port,default=1883,minimum=1,maximum=65535,example=1883"`
	Username          string                 `yaml:"username,omitempty" json:"username,omitempty" jsonschema:"title=Username,description=Username for remote broker authentication. Supports env vars,example=${CLOUD_USER}"`
	Password          string                 `yaml:"password,omitempty" json:"password,omitempty" jsonschema:"title=Password,description=Password for remote broker authentication. Supports env vars,example=${CLOUD_PASSWORD}"`
	ClientID          string                 `yaml:"client_id,omitempty" json:"client_id,omitempty" jsonschema:"title=Client ID,description=MQTT client ID for bridge connection,example=edge-broker-001"`
	CleanSession      bool                   `yaml:"clean_session,omitempty" json:"clean_session,omitempty" jsonschema:"title=Clean Session,description=Start with clean session (true) or resume previous session (false),default=true"`
	KeepAlive         int                    `yaml:"keep_alive,omitempty" json:"keep_alive,omitempty" jsonschema:"title=Keep Alive,description=Keep alive interval in seconds,default=60,minimum=1,example=60"`
	ConnectionTimeout int                    `yaml:"connection_timeout,omitempty" json:"connection_timeout,omitempty" jsonschema:"title=Connection Timeout,description=Connection timeout in seconds,default=30,minimum=1,example=30"`
	Metadata          map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty" jsonschema:"title=Metadata,description=Custom metadata key-value pairs"`
	Topics            []BridgeTopicConfig    `yaml:"topics" json:"topics" jsonschema:"required,title=Topic Mappings,description=Topic mappings for message forwarding,minItems=1"`
}

// BridgeTopicConfig represents a topic mapping in a bridge configuration
type BridgeTopicConfig struct {
	Local     string `yaml:"local" json:"local" jsonschema:"required,title=Local Topic,description=Local topic pattern to match messages,minLength=1,example=sensors/#"`
	Remote    string `yaml:"remote" json:"remote" jsonschema:"required,title=Remote Topic,description=Remote topic pattern for forwarding,minLength=1,example=edge/sensors/#"`
	Direction string `yaml:"direction" json:"direction" jsonschema:"required,title=Direction,description=Message forwarding direction,enum=in,enum=out,enum=both,example=out"`
	QoS       int    `yaml:"qos,omitempty" json:"qos,omitempty" jsonschema:"title=QoS,description=MQTT Quality of Service level,default=0,minimum=0,maximum=2,example=1"`
}

// ScriptConfig represents a script in the config file
type ScriptConfig struct {
	Name        string                 `yaml:"name" json:"name" jsonschema:"required,title=Script Name,description=Unique name for this script,minLength=1,example=message-logger"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"title=Description,description=Human-readable description,example=Log all published messages"`
	Enabled     bool                   `yaml:"enabled" json:"enabled" jsonschema:"title=Enabled,description=Whether this script is active,default=true"`
	File        string                 `yaml:"file,omitempty" json:"file,omitempty" jsonschema:"title=Script File,description=Path to JavaScript file. Supports env vars. Mutually exclusive with content,example=./scripts/logger.js"`
	Content     string                 `yaml:"content,omitempty" json:"content,omitempty" jsonschema:"title=Script Content,description=Inline JavaScript code. Supports env vars (${API_KEY}) and $$ escaping for JS templates ($${var}). Mutually exclusive with file,example=log.info('Message:', msg.topic);"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty" jsonschema:"title=Metadata,description=Custom metadata key-value pairs accessible in script"`
	Triggers    []ScriptTriggerConfig  `yaml:"triggers" json:"triggers" jsonschema:"required,title=Triggers,description=When this script should execute,minItems=1"`
}

// ScriptTriggerConfig represents a trigger for a script
type ScriptTriggerConfig struct {
	Type     string `yaml:"type" json:"type" jsonschema:"required,title=Trigger Type,description=MQTT event type that triggers this script,enum=on_publish,enum=on_connect,enum=on_disconnect,enum=on_subscribe,example=on_publish"`
	Topic    string `yaml:"topic,omitempty" json:"topic,omitempty" jsonschema:"title=Topic Filter,description=MQTT topic pattern to filter events (empty = all topics). Supports wildcards (+/#),example=#"`
	Priority int    `yaml:"priority,omitempty" json:"priority,omitempty" jsonschema:"title=Priority,description=Execution order (lower = earlier). Default: 100,default=100,minimum=0,example=50"`
	Enabled  bool   `yaml:"enabled" json:"enabled" jsonschema:"title=Enabled,description=Whether this trigger is active,default=true"`
}

// reservedPlaceholders lists variable names that should never be expanded as env vars
// These are runtime placeholders used in ACL rules and other MQTT contexts
var reservedPlaceholders = []string{
	"username", // ACL placeholder - replaced at runtime with MQTT username
	"clientid", // ACL placeholder - replaced at runtime with MQTT client ID
	// Add more reserved placeholders here as needed
}

// isReservedPlaceholder checks if a variable name is a reserved placeholder
func isReservedPlaceholder(name string) bool {
	for _, reserved := range reservedPlaceholders {
		if name == reserved {
			return true
		}
	}
	return false
}

// customMapper is used by os.Expand to handle environment variable expansion
// Supports:
// - ${username} and ${clientid} - preserved as ACL/MQTT placeholders
// - ${VAR:-default} - env var with default value (Docker Compose style)
// - ${VAR} - standard env var expansion
func customMapper(name string) string {
	// Preserve reserved runtime placeholders - never expand these
	if isReservedPlaceholder(name) {
		return "${" + name + "}"
	}

	// Handle default value syntax: ${VAR:-default}
	if strings.Contains(name, ":-") {
		parts := strings.SplitN(name, ":-", 2)
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			defaultVal := parts[1] // Don't trim - preserve whitespace in default

			// Return env var if set and non-empty, otherwise use default
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return defaultVal
		}
	}

	// Standard env var expansion
	return os.Getenv(name)
}

// escapeDollarSigns protects $$ (double dollar) from expansion
// $$ becomes a temporary marker that will be restored to $ after expansion
func escapeDollarSigns(content string) string {
	return strings.ReplaceAll(content, "$$", "__ESCAPED_DOLLAR__")
}

// restoreDollarSigns converts markers back to literal $
func restoreDollarSigns(content string) string {
	return strings.ReplaceAll(content, "__ESCAPED_DOLLAR__", "$")
}

// Load reads and parses a YAML config file with environment variable interpolation
// Supports Docker Compose-style syntax:
// - ${VAR} - expand environment variable (empty string if unset)
// - ${VAR:-default} - expand env var with default value if unset/empty
// - ${username} and ${clientid} - preserved as ACL/MQTT runtime placeholders
// - $${...} - escaped, becomes literal ${...} (for JavaScript template literals)
func Load(path string) (*Config, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	content := string(data)

	// Step 1: Protect $$ (escaped dollar signs) from expansion
	// $$ → __ESCAPED_DOLLAR__ → (after expansion) → $
	content = escapeDollarSigns(content)

	// Step 2: Expand environment variables using custom mapper
	// Mapper handles: ${username}, ${clientid}, ${VAR:-default}, ${VAR}
	expanded := os.Expand(content, customMapper)

	// Step 3: Restore escaped dollar signs
	expanded = restoreDollarSigns(expanded)

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
		if rule.Username == "" {
			return fmt.Errorf("ACL rule missing username")
		}
		if rule.Topic == "" {
			return fmt.Errorf("ACL rule for user '%s' missing topic", rule.Username)
		}
		if rule.Permission == "" {
			return fmt.Errorf("ACL rule for user '%s' missing permission", rule.Username)
		}

		// Check if username exists
		if !validUsernames[rule.Username] {
			return fmt.Errorf("ACL rule references unknown user: %s", rule.Username)
		}

		// Validate permission
		if rule.Permission != "pub" && rule.Permission != "sub" && rule.Permission != "pubsub" {
			return fmt.Errorf("ACL rule for user '%s' has invalid permission: %s (must be pub, sub, or pubsub)", rule.Username, rule.Permission)
		}
	}

	// Validate bridges
	bridgeNames := make(map[string]bool)
	for _, bridge := range c.Bridges {
		if bridge.Name == "" {
			return fmt.Errorf("bridge missing name")
		}
		if bridge.Host == "" {
			return fmt.Errorf("bridge '%s' missing host", bridge.Name)
		}
		if bridgeNames[bridge.Name] {
			return fmt.Errorf("duplicate bridge name: %s", bridge.Name)
		}
		bridgeNames[bridge.Name] = true

		// Set defaults
		if bridge.Port == 0 {
			bridge.Port = 1883
		}
		if bridge.Port < 1 || bridge.Port > 65535 {
			return fmt.Errorf("bridge '%s' has invalid port: %d", bridge.Name, bridge.Port)
		}

		// Validate topics
		if len(bridge.Topics) == 0 {
			return fmt.Errorf("bridge '%s' has no topics configured", bridge.Name)
		}
		for _, topic := range bridge.Topics {
			if topic.Local == "" {
				return fmt.Errorf("bridge '%s' has topic with empty local", bridge.Name)
			}
			if topic.Remote == "" {
				return fmt.Errorf("bridge '%s' has topic with empty remote", bridge.Name)
			}
			if topic.Direction != "in" && topic.Direction != "out" && topic.Direction != "both" {
				return fmt.Errorf("bridge '%s' has invalid direction '%s' (must be in, out, or both)", bridge.Name, topic.Direction)
			}
			if topic.QoS < 0 || topic.QoS > 2 {
				return fmt.Errorf("bridge '%s' has invalid QoS %d (must be 0, 1, or 2)", bridge.Name, topic.QoS)
			}
		}
	}

	// Validate scripts
	scriptNames := make(map[string]bool)
	for _, script := range c.Scripts {
		if script.Name == "" {
			return fmt.Errorf("script missing name")
		}
		if scriptNames[script.Name] {
			return fmt.Errorf("duplicate script name: %s", script.Name)
		}
		scriptNames[script.Name] = true

		// Must have either file or content, but not both
		hasFile := script.File != ""
		hasContent := script.Content != ""
		if !hasFile && !hasContent {
			return fmt.Errorf("script '%s' must have either file or content", script.Name)
		}
		if hasFile && hasContent {
			return fmt.Errorf("script '%s' cannot have both file and content", script.Name)
		}

		// Validate triggers
		if len(script.Triggers) == 0 {
			return fmt.Errorf("script '%s' has no triggers configured", script.Name)
		}
		for i, trigger := range script.Triggers {
			if trigger.Type == "" {
				return fmt.Errorf("script '%s' trigger %d missing type", script.Name, i+1)
			}
			// Validate trigger type
			validTriggers := []string{"on_publish", "on_connect", "on_disconnect", "on_subscribe"}
			valid := false
			for _, vt := range validTriggers {
				if trigger.Type == vt {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("script '%s' has invalid type '%s' (must be one of: on_publish, on_connect, on_disconnect, on_subscribe)", script.Name, trigger.Type)
			}

			// Set default priority
			if trigger.Priority == 0 {
				script.Triggers[i].Priority = 100
			}
		}
	}

	return nil
}
