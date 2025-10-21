package api

import "github/bherbruck/mqtt-server/internal/storage"

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response with JWT token
type LoginResponse struct {
	Token string        `json:"token"`
	User  *storage.User `json:"user"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UpdatePasswordRequest represents a request to update a user's password
type UpdatePasswordRequest struct {
	Password string `json:"password"`
}

// CreateACLRequest represents a request to create an ACL rule
type CreateACLRequest struct {
	UserID       int    `json:"user_id"`
	TopicPattern string `json:"topic_pattern"`
	Permission   string `json:"permission"`
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
