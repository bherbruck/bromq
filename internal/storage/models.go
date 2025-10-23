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
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"not null" json:"-"` // Never expose password hash in JSON
	Description  string         `gorm:"type:text" json:"description"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"` // Custom attributes
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
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
	ID           uint      `gorm:"primaryKey" json:"id"`
	MQTTUserID   uint      `gorm:"uniqueIndex:idx_acl_user_topic;not null" json:"mqtt_user_id"`
	TopicPattern string    `gorm:"uniqueIndex:idx_acl_user_topic;not null" json:"topic_pattern"`
	Permission   string    `gorm:"not null;check:permission IN ('pub', 'sub', 'pubsub')" json:"permission"`
	CreatedAt    time.Time `json:"created_at"`
	MQTTUser     MQTTUser  `gorm:"foreignKey:MQTTUserID;constraint:OnDelete:CASCADE" json:"-"`
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
