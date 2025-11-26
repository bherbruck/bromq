package storage

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DashboardUser represents a web dashboard user (human user who logs into the web interface)
type DashboardUser struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"not null" json:"-"` // Never expose password hash in JSON
	Role         string         `gorm:"not null;default:viewer" json:"role"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"` // Custom attributes
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// TableName specifies the table name for DashboardUser model
func (DashboardUser) TableName() string {
	return "dashboard_users"
}

// MQTTUser represents MQTT authentication credentials (can be shared by multiple devices)
type MQTTUser struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	Username             string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash         string         `gorm:"not null" json:"-"` // Never expose password hash in JSON
	Description          string         `gorm:"type:text" json:"description"`
	Metadata             datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"` // Custom attributes
	ProvisionedFromConfig bool          `gorm:"default:false" json:"provisioned_from_config"` // Managed by config file
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// TableName specifies the table name for MQTTUser model
func (MQTTUser) TableName() string {
	return "mqtt_users"
}

// GetID returns the user ID for the tracking hook
func (u *MQTTUser) GetID() uint {
	return u.ID
}

// MQTTClient represents an individual MQTT device/client connection
// Multiple clients can use the same MQTTUser credentials
type MQTTClient struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ClientID   string         `gorm:"uniqueIndex;not null" json:"client_id"` // MQTT Client ID
	MQTTUserID uint           `gorm:"index:idx_mqtt_client_user;not null" json:"mqtt_user_id"`
	Metadata   datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"` // Custom attributes per device
	FirstSeen  time.Time      `gorm:"not null" json:"first_seen"`
	LastSeen   time.Time      `gorm:"not null" json:"last_seen"`
	IsActive   bool           `gorm:"default:false" json:"is_active"` // Currently connected
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	MQTTUser   MQTTUser       `gorm:"foreignKey:MQTTUserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName specifies the table name for MQTTClient model
func (MQTTClient) TableName() string {
	return "mqtt_clients"
}

// ACLRule represents an access control rule for MQTT topics
// Rules are associated with MQTTUser (credentials), not individual clients
type ACLRule struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	MQTTUserID            uint      `gorm:"uniqueIndex:idx_acl_user_topic;not null" json:"mqtt_user_id"`
	Topic                 string    `gorm:"uniqueIndex:idx_acl_user_topic;not null" json:"topic"`
	Permission            string    `gorm:"not null;check:permission IN ('pub', 'sub', 'pubsub')" json:"permission"`
	ProvisionedFromConfig bool      `gorm:"default:false" json:"provisioned_from_config"` // Managed by config file
	CreatedAt             time.Time `json:"created_at"`
	MQTTUser              MQTTUser  `gorm:"foreignKey:MQTTUserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName specifies the table name for ACLRule model
func (ACLRule) TableName() string {
	return "acl_rules"
}

// RetainedMessage represents a retained MQTT message stored in the database
type RetainedMessage struct {
	Topic     string    `gorm:"primaryKey;index:idx_retained_topic" json:"topic"`
	Payload   []byte    `gorm:"not null" json:"payload"`
	QoS       byte      `gorm:"column:qos;not null" json:"qos"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for RetainedMessage model
func (RetainedMessage) TableName() string {
	return "retained_messages"
}

// BeforeCreate hook for DashboardUser to ensure role is set
func (u *DashboardUser) BeforeCreate(tx *gorm.DB) error {
	if u.Role == "" {
		u.Role = "viewer"
	}
	return nil
}

// BeforeCreate hook for MQTTClient to set timestamps
func (c *MQTTClient) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if c.FirstSeen.IsZero() {
		c.FirstSeen = now
	}
	if c.LastSeen.IsZero() {
		c.LastSeen = now
	}
	return nil
}

// Bridge represents an MQTT bridge connection to a remote broker
type Bridge struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	Name                  string         `gorm:"uniqueIndex;not null" json:"name"`
	Host                  string         `gorm:"not null" json:"host"`
	Port                  int            `gorm:"not null;default:1883" json:"port"`
	Username              string         `gorm:"default:''" json:"username"`
	Password              string         `gorm:"default:''" json:"-"` // Plain text, needed for outbound connections
	ClientID              string         `gorm:"default:''" json:"client_id"`
	MQTTVersion           string         `gorm:"default:'5';check:mqtt_version IN ('3', '5')" json:"mqtt_version"` // MQTT protocol version: "3" (3.1.1) or "5"
	CleanSession          bool           `gorm:"default:true" json:"clean_session"`                                 // v3: CleanSession, v5: CleanStart
	KeepAlive             int            `gorm:"default:60" json:"keep_alive"`                                      // seconds
	ConnectionTimeout     int            `gorm:"default:30" json:"connection_timeout"`                              // seconds
	ProvisionedFromConfig bool           `gorm:"default:false" json:"provisioned_from_config"`
	Metadata              datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	Topics                []BridgeTopic  `gorm:"foreignKey:BridgeID;constraint:OnDelete:CASCADE" json:"topics,omitempty"`
}

// TableName specifies the table name for Bridge model
func (Bridge) TableName() string {
	return "bridges"
}

// BridgeTopic represents a topic mapping for an MQTT bridge
type BridgeTopic struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BridgeID  uint      `gorm:"not null;index" json:"bridge_id"`
	Local     string    `gorm:"not null" json:"local"`
	Remote    string    `gorm:"not null" json:"remote"`
	Direction string    `gorm:"not null;default:'out';check:direction IN ('in', 'out', 'both')" json:"direction"`
	QoS       byte      `gorm:"column:qos;not null;default:0" json:"qos"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for BridgeTopic model
func (BridgeTopic) TableName() string {
	return "bridge_topics"
}

// Script represents a JavaScript script that executes on MQTT events
type Script struct {
	ID                    uint            `gorm:"primaryKey" json:"id"`
	Name                  string          `gorm:"uniqueIndex;not null" json:"name"`
	Description           string          `gorm:"type:text" json:"description"`
	Content               string          `gorm:"type:text;not null" json:"content"`
	Enabled               bool            `gorm:"default:true" json:"enabled"`
	TimeoutSeconds        *int            `gorm:"default:null" json:"timeout_seconds,omitempty"` // Script execution timeout in seconds (null = use default)
	ProvisionedFromConfig bool            `gorm:"default:false" json:"provisioned_from_config"`
	Metadata              datatypes.JSON  `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	Triggers              []ScriptTrigger `gorm:"foreignKey:ScriptID;constraint:OnDelete:CASCADE" json:"triggers,omitempty"`
}

// TableName specifies the table name for Script model
func (Script) TableName() string {
	return "scripts"
}

// ScriptTrigger defines when a script should execute
type ScriptTrigger struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ScriptID  uint      `gorm:"not null;index:idx_script_trigger" json:"script_id"`
	Type      string    `gorm:"not null;index:idx_script_trigger;check:type IN ('on_publish', 'on_connect', 'on_disconnect', 'on_subscribe')" json:"type"`
	Topic     string    `gorm:"default:''" json:"topic"` // MQTT topic pattern (empty for non-topic events)
	Priority  int       `gorm:"default:100" json:"priority"` // Execution order (lower = earlier)
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for ScriptTrigger model
func (ScriptTrigger) TableName() string {
	return "script_triggers"
}

// ScriptLog stores script execution logs
// DEPRECATED: This model is no longer used - script logs are now stored in BadgerDB
// for better write performance. See internal/badgerstore/script_logs.go
// This struct is kept for backward compatibility with old database schemas.
type ScriptLog struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ScriptID        uint           `gorm:"not null;index:idx_script_log_timestamp" json:"script_id"`
	Type            string         `gorm:"not null" json:"type"`
	Level           string         `gorm:"not null;check:level IN ('debug', 'info', 'warn', 'error')" json:"level"`
	Message         string         `gorm:"type:text" json:"message"`
	Context         datatypes.JSON `gorm:"type:jsonb" json:"context,omitempty"` // Client ID, topic, etc.
	ExecutionTimeMs int            `json:"execution_time_ms"`
	CreatedAt       time.Time      `gorm:"index:idx_script_log_timestamp" json:"created_at"`
	Script          Script         `gorm:"foreignKey:ScriptID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName specifies the table name for ScriptLog model
func (ScriptLog) TableName() string {
	return "script_logs"
}

// ScriptState stores persistent state for scripts (key-value store)
type ScriptState struct {
	Key       string     `gorm:"primaryKey;size:255" json:"key"` // Format: "script:{id}:{userkey}" or "global:{userkey}"
	ScriptID  *uint      `gorm:"index" json:"script_id"`         // NULL for global state
	Value     []byte     `gorm:"type:bytea" json:"value"`        // JSON-encoded value
	ExpiresAt *time.Time `gorm:"index" json:"expires_at"`        // NULL = no expiration
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName specifies the table name for ScriptState model
func (ScriptState) TableName() string {
	return "script_state"
}
