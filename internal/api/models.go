package api

import (
	"github/bherbruck/bromq/internal/storage"
	"gorm.io/datatypes"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response with JWT token
type LoginResponse struct {
	Token string              `json:"token"`
	User  *storage.DashboardUser `json:"user"`
}

// === Admin User Requests ===

// CreateDashboardUserRequest represents a request to create a new admin user
type CreateDashboardUserRequest struct {
	Username string         `json:"username"`
	Password string         `json:"password"`
	Role     string         `json:"role"`
	Metadata datatypes.JSON `json:"metadata,omitempty"`
}

// UpdateDashboardUserRequest represents a request to update an admin user
type UpdateDashboardUserRequest struct {
	Username string         `json:"username"`
	Role     string         `json:"role"`
	Metadata datatypes.JSON `json:"metadata,omitempty"`
}

// UpdateAdminPasswordRequest represents a request to update an admin's password
type UpdateAdminPasswordRequest struct {
	Password string `json:"password"`
}

// ChangePasswordRequest represents a request for a user to change their own password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// === MQTT User (Credentials) Requests ===

// CreateMQTTUserRequest represents a request to create MQTT credentials
type CreateMQTTUserRequest struct {
	Username    string         `json:"username"`
	Password    string         `json:"password"`
	Description string         `json:"description"`
	Metadata    datatypes.JSON `json:"metadata,omitempty"`
}

// UpdateMQTTUserRequest represents a request to update MQTT credentials
type UpdateMQTTUserRequest struct {
	Username    string         `json:"username"`
	Description string         `json:"description"`
	Metadata    datatypes.JSON `json:"metadata,omitempty"`
}

// UpdateMQTTPasswordRequest represents a request to update MQTT credentials password
type UpdateMQTTPasswordRequest struct {
	Password string `json:"password"`
}

// === MQTT Client Requests ===

// UpdateMQTTClientMetadataRequest represents a request to update client metadata
type UpdateMQTTClientMetadataRequest struct {
	Metadata datatypes.JSON `json:"metadata"`
}

// CreateACLRequest represents a request to create an ACL rule
type CreateACLRequest struct {
	MQTTUserID   int    `json:"mqtt_user_id"`
	TopicPattern string `json:"topic_pattern"`
	Permission   string `json:"permission"`
}

// UpdateACLRequest represents a request to update an ACL rule
type UpdateACLRequest struct {
	TopicPattern string `json:"topic_pattern"`
	Permission   string `json:"permission"`
}

// === Bridge Requests ===

// BridgeTopicRequest represents a topic mapping for a bridge
type BridgeTopicRequest struct {
	LocalPattern  string `json:"local_pattern"`
	RemotePattern string `json:"remote_pattern"`
	Direction     string `json:"direction"` // "in", "out", or "both"
	QoS           byte   `json:"qos"`
}

// CreateBridgeRequest represents a request to create a bridge
type CreateBridgeRequest struct {
	Name              string                 `json:"name"`
	RemoteHost        string                 `json:"remote_host"`
	RemotePort        int                    `json:"remote_port"`
	RemoteUsername    string                 `json:"remote_username,omitempty"`
	RemotePassword    string                 `json:"remote_password,omitempty"`
	ClientID          string                 `json:"client_id,omitempty"`
	CleanSession      bool                   `json:"clean_session"`
	KeepAlive         int                    `json:"keep_alive"`
	ConnectionTimeout int                    `json:"connection_timeout"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Topics            []BridgeTopicRequest   `json:"topics"`
}

// UpdateBridgeRequest represents a request to update a bridge
type UpdateBridgeRequest struct {
	Name              string                 `json:"name"`
	RemoteHost        string                 `json:"remote_host"`
	RemotePort        int                    `json:"remote_port"`
	RemoteUsername    string                 `json:"remote_username,omitempty"`
	RemotePassword    string                 `json:"remote_password,omitempty"`
	ClientID          string                 `json:"client_id,omitempty"`
	CleanSession      bool                   `json:"clean_session"`
	KeepAlive         int                    `json:"keep_alive"`
	ConnectionTimeout int                    `json:"connection_timeout"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Topics            []BridgeTopicRequest   `json:"topics"`
}

// PaginationQuery represents pagination query parameters
type PaginationQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Search   string `json:"search"`
	SortBy   string `json:"sort_by"`
	SortOrder string `json:"sort_order"` // "asc" or "desc"
}

// PaginationMetadata represents pagination metadata in responses
type PaginationMetadata struct {
	Total       int64 `json:"total"`
	Page        int   `json:"page"`
	PageSize    int   `json:"page_size"`
	TotalPages  int   `json:"total_pages"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{}         `json:"data"`
	Pagination PaginationMetadata  `json:"pagination"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
