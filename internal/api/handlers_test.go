package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github/bherbruck/mqtt-server/internal/mqtt"
	"github/bherbruck/mqtt-server/internal/storage"
)

// setupTestHandler creates a handler with an in-memory database and mock MQTT server
func setupTestHandler(t *testing.T) (*Handler, *storage.DB, func()) {
	t.Helper()

	// Create in-memory database
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create mock MQTT server
	mqttServer := mqtt.New(nil)

	handler := NewHandler(db, mqttServer)

	cleanup := func() {
		db.Close()
	}

	return handler, db, cleanup
}

func TestLogin(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test user
	_, err := db.CreateUser("testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	tests := []struct {
		name           string
		requestBody    interface{}
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful login",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp LoginResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp.Token == "" {
					t.Errorf("Login() token is empty")
				}
				if resp.User == nil {
					t.Errorf("Login() user is nil")
				} else if resp.User.Username != "testuser" {
					t.Errorf("Login() username = %v, want testuser", resp.User.Username)
				}
			},
		},
		{
			name: "login with default admin",
			requestBody: LoginRequest{
				Username: "admin",
				Password: "admin",
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp LoginResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp.Token == "" {
					t.Errorf("Login() token is empty")
				}
			},
		},
		{
			name: "login with wrong password",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "wrongpassword",
			},
			wantStatusCode: http.StatusUnauthorized,
			checkResponse:  nil,
		},
		{
			name: "login with non-existent user",
			requestBody: LoginRequest{
				Username: "nonexistent",
				Password: "password123",
			},
			wantStatusCode: http.StatusUnauthorized,
			checkResponse:  nil,
		},
		{
			name:           "login with invalid JSON",
			requestBody:    "invalid json",
			wantStatusCode: http.StatusBadRequest,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.Login(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("Login() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test users
	db.CreateUser("user1", "password123", "user")
	db.CreateUser("user2", "password123", "user")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()

	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListUsers() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var users []storage.User
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should have 3 users (2 created + 1 default admin)
	if len(users) != 3 {
		t.Errorf("ListUsers() returned %d users, want 3", len(users))
	}
}

func TestCreateUser(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name           string
		requestBody    interface{}
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "create valid user",
			requestBody: CreateUserRequest{
				Username: "newuser",
				Password: "password123",
				Role:     "user",
			},
			wantStatusCode: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var user storage.User
				if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if user.Username != "newuser" {
					t.Errorf("CreateUser() username = %v, want newuser", user.Username)
				}
				if user.Role != "user" {
					t.Errorf("CreateUser() role = %v, want user", user.Role)
				}
			},
		},
		{
			name: "create user with invalid role",
			requestBody: CreateUserRequest{
				Username: "newuser2",
				Password: "password123",
				Role:     "superadmin",
			},
			wantStatusCode: http.StatusInternalServerError,
			checkResponse:  nil,
		},
		{
			name:           "create user with invalid JSON",
			requestBody:    "invalid json",
			wantStatusCode: http.StatusBadRequest,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.CreateUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("CreateUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test user
	user, err := db.CreateUser("testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	tests := []struct {
		name           string
		userID         string
		requestBody    interface{}
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "update user successfully",
			userID: fmt.Sprintf("%d", user.ID),
			requestBody: UpdateUserRequest{
				Username: "updateduser",
				Role:     "admin",
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var updatedUser storage.User
				if err := json.Unmarshal(rec.Body.Bytes(), &updatedUser); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if updatedUser.Username != "updateduser" {
					t.Errorf("UpdateUser() username = %v, want updateduser", updatedUser.Username)
				}
				if updatedUser.Role != "admin" {
					t.Errorf("UpdateUser() role = %v, want admin", updatedUser.Role)
				}
			},
		},
		{
			name:   "update non-existent user",
			userID: "999999",
			requestBody: UpdateUserRequest{
				Username: "ghost",
				Role:     "user",
			},
			wantStatusCode: http.StatusInternalServerError,
			checkResponse:  nil,
		},
		{
			name:           "update with invalid user ID",
			userID:         "invalid",
			requestBody:    UpdateUserRequest{},
			wantStatusCode: http.StatusBadRequest,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/users/%s", tt.userID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", tt.userID)

			rec := httptest.NewRecorder()
			handler.UpdateUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test user
	user, err := db.CreateUser("todelete", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	tests := []struct {
		name           string
		userID         string
		wantStatusCode int
	}{
		{
			name:           "delete existing user",
			userID:         fmt.Sprintf("%d", user.ID),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "delete non-existent user",
			userID:         "999999",
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "delete with invalid user ID",
			userID:         "invalid",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/users/%s", tt.userID), nil)
			req.SetPathValue("id", tt.userID)

			rec := httptest.NewRecorder()
			handler.DeleteUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestListACL(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test user and ACL rules
	user, _ := db.CreateUser("testuser", "password123", "user")
	db.CreateACLRule(user.ID, "devices/+/telemetry", "pub")
	db.CreateACLRule(user.ID, "commands/#", "sub")

	req := httptest.NewRequest(http.MethodGet, "/api/acl", nil)
	rec := httptest.NewRecorder()

	handler.ListACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListACL() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var rules []storage.ACLRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(rules) != 2 {
		t.Errorf("ListACL() returned %d rules, want 2", len(rules))
	}
}

func TestCreateACL(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test user
	user, _ := db.CreateUser("testuser", "password123", "user")

	tests := []struct {
		name           string
		requestBody    interface{}
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "create valid ACL rule",
			requestBody: CreateACLRequest{
				UserID:       user.ID,
				TopicPattern: "devices/+/telemetry",
				Permission:   "pub",
			},
			wantStatusCode: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var rule storage.ACLRule
				if err := json.Unmarshal(rec.Body.Bytes(), &rule); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if rule.TopicPattern != "devices/+/telemetry" {
					t.Errorf("CreateACL() topicPattern = %v, want devices/+/telemetry", rule.TopicPattern)
				}
			},
		},
		{
			name: "create ACL rule with invalid permission",
			requestBody: CreateACLRequest{
				UserID:       user.ID,
				TopicPattern: "test/topic",
				Permission:   "readwrite",
			},
			wantStatusCode: http.StatusInternalServerError,
			checkResponse:  nil,
		},
		{
			name:           "create ACL with invalid JSON",
			requestBody:    "invalid json",
			wantStatusCode: http.StatusBadRequest,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/acl", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.CreateACL(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("CreateACL() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestDeleteACL(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test user and ACL rule
	user, _ := db.CreateUser("testuser", "password123", "user")
	rule, _ := db.CreateACLRule(user.ID, "devices/+/telemetry", "pub")

	tests := []struct {
		name           string
		ruleID         string
		wantStatusCode int
	}{
		{
			name:           "delete existing rule",
			ruleID:         fmt.Sprintf("%d", rule.ID),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "delete non-existent rule",
			ruleID:         "999999",
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "delete with invalid rule ID",
			ruleID:         "invalid",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/acl/%s", tt.ruleID), nil)
			req.SetPathValue("id", tt.ruleID)

			rec := httptest.NewRecorder()
			handler.DeleteACL(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteACL() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestListClients(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	rec := httptest.NewRecorder()

	handler.ListClients(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListClients() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var clients []mqtt.ClientInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &clients); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should return empty array for new server
	if clients == nil {
		t.Errorf("ListClients() returned nil instead of empty array")
	}
}

func TestGetMetrics(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Give server a moment to initialize
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	rec := httptest.NewRecorder()

	handler.GetMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetMetrics() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var metrics mqtt.Metrics
	if err := json.Unmarshal(rec.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Basic sanity checks
	if metrics.Uptime < 0 {
		t.Errorf("GetMetrics() uptime should not be negative")
	}
}

func TestGetClientDetails(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name           string
		clientID       string
		wantStatusCode int
	}{
		{
			name:           "get non-existent client",
			clientID:       "nonexistent",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "empty client ID",
			clientID:       "",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/clients/%s", tt.clientID), nil)
			req.SetPathValue("id", tt.clientID)

			rec := httptest.NewRecorder()
			handler.GetClientDetails(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("GetClientDetails() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestDisconnectClient(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name           string
		clientID       string
		wantStatusCode int
	}{
		{
			name:           "disconnect non-existent client",
			clientID:       "nonexistent",
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "empty client ID",
			clientID:       "",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/clients/%s/disconnect", tt.clientID), nil)
			req.SetPathValue("id", tt.clientID)

			rec := httptest.NewRecorder()
			handler.DisconnectClient(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DisconnectClient() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}
