package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github/bherbruck/bromq/internal/storage"
)

// MockMQTTServer is a simple mock for testing
type MockMQTTServer struct{}

func (m *MockMQTTServer) GetClients() []interface{} {
	return []interface{}{}
}

func (m *MockMQTTServer) GetClientDetails(clientID string) (interface{}, error) {
	return nil, fmt.Errorf("client not found")
}

func (m *MockMQTTServer) DisconnectClient(clientID string) error {
	return nil
}

func (m *MockMQTTServer) GetMetrics() interface{} {
	return map[string]interface{}{
		"clients":  0,
		"messages": 0,
	}
}

// setupTestHandler creates a test handler with in-memory database and mock MQTT server
func setupTestHandler(t *testing.T) *Handler {
	// Create in-memory test database
	dbConfig := storage.DefaultSQLiteConfig(":memory:")
	// Use isolated Prometheus registry to prevent duplicate registration in tests
	cache := storage.NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := storage.OpenWithCache(dbConfig, cache)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create test config with JWT secret
	testConfig := &Config{
		JWTSecret: []byte("test-jwt-secret-for-testing-only"),
	}

	// Create a mock MQTT server that implements the needed interface
	// We'll cast it to *mqtt.Server for compatibility
	// In reality, the handlers should use an interface, but for testing we use a workaround
	return &Handler{
		db:     db,
		mqtt:   nil, // Use nil for now, handlers that need MQTT will be skipped
		engine: nil, // No script engine needed for basic tests
		config: testConfig,
	}
}

func TestLogin(t *testing.T) {
	handler := setupTestHandler(t)

	tests := []struct {
		name           string
		request        LoginRequest
		wantStatusCode int
		wantToken      bool
	}{
		{
			name: "successful login with default admin",
			request: LoginRequest{
				Username: "admin",
				Password: "admin",
			},
			wantStatusCode: http.StatusOK,
			wantToken:      true,
		},
		{
			name: "invalid password",
			request: LoginRequest{
				Username: "admin",
				Password: "wrongpassword",
			},
			wantStatusCode: http.StatusUnauthorized,
			wantToken:      false,
		},
		{
			name: "non-existent user",
			request: LoginRequest{
				Username: "nonexistent",
				Password: "password",
			},
			wantStatusCode: http.StatusUnauthorized,
			wantToken:      false,
		},
		{
			name: "empty username",
			request: LoginRequest{
				Username: "",
				Password: "password",
			},
			wantStatusCode: http.StatusUnauthorized,
			wantToken:      false,
		},
		{
			name: "empty password",
			request: LoginRequest{
				Username: "admin",
				Password: "",
			},
			wantStatusCode: http.StatusUnauthorized,
			wantToken:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("Login() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response body: %s", rec.Body.String())
			}

			if tt.wantToken {
				var response LoginResponse
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Token == "" {
					t.Errorf("Login() expected token but got empty")
				}

				if response.User == nil {
					t.Errorf("Login() expected user but got nil")
				}

				if response.User != nil && response.User.Username != tt.request.Username {
					t.Errorf("Login() username = %v, want %v", response.User.Username, tt.request.Username)
				}
			}
		})
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Login() with invalid JSON status = %v, want %v", rec.Code, http.StatusBadRequest)
	}
}

func TestListACL(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test data
	mqttUser, err := handler.db.CreateMQTTUser("testuser", "password123", "Test user", nil)
	if err != nil {
		t.Fatalf("Failed to create test MQTT user: %v", err)
	}

	rule1, err := handler.db.CreateACLRule(int(mqttUser.ID), "sensor/#", "pubsub")
	if err != nil {
		t.Fatalf("Failed to create test ACL rule: %v", err)
	}

	rule2, err := handler.db.CreateACLRule(int(mqttUser.ID), "device/+/status", "pub")
	if err != nil {
		t.Fatalf("Failed to create second test ACL rule: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/acl", nil)
	rec := httptest.NewRecorder()

	handler.ListACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListACL() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response struct {
		Data       []storage.ACLRule  `json:"data"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("ListACL() returned %d rules, want 2", len(response.Data))
	}

	// Verify rule content
	foundRule1 := false
	foundRule2 := false
	for _, rule := range response.Data {
		if rule.ID == rule1.ID && rule.TopicPattern == "sensor/#" {
			foundRule1 = true
		}
		if rule.ID == rule2.ID && rule.TopicPattern == "device/+/status" {
			foundRule2 = true
		}
	}

	if !foundRule1 {
		t.Errorf("ListACL() did not return rule1")
	}
	if !foundRule2 {
		t.Errorf("ListACL() did not return rule2")
	}
}

func TestListACL_Empty(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/acl", nil)
	rec := httptest.NewRecorder()

	handler.ListACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListACL() empty status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response struct {
		Data       []storage.ACLRule  `json:"data"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Data == nil {
		t.Errorf("ListACL() returned nil data, should return empty array")
	}

	if len(response.Data) != 0 {
		t.Errorf("ListACL() empty returned %d rules, want 0", len(response.Data))
	}
}

func TestCreateACL(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test MQTT user
	mqttUser, err := handler.db.CreateMQTTUser("testuser", "password123", "Test user", nil)
	if err != nil {
		t.Fatalf("Failed to create test MQTT user: %v", err)
	}

	tests := []struct {
		name           string
		request        CreateACLRequest
		wantStatusCode int
	}{
		{
			name: "create valid ACL rule",
			request: CreateACLRequest{
				MQTTUserID:   int(mqttUser.ID),
				TopicPattern: "sensor/temperature",
				Permission:   "pubsub",
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "create pub-only rule",
			request: CreateACLRequest{
				MQTTUserID:   int(mqttUser.ID),
				TopicPattern: "device/status",
				Permission:   "pub",
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "create sub-only rule",
			request: CreateACLRequest{
				MQTTUserID:   int(mqttUser.ID),
				TopicPattern: "command/#",
				Permission:   "sub",
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "create with wildcard topic",
			request: CreateACLRequest{
				MQTTUserID:   int(mqttUser.ID),
				TopicPattern: "sensor/+/temp",
				Permission:   "pubsub",
			},
			wantStatusCode: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/acl", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.CreateACL(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("CreateACL() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response body: %s", rec.Body.String())
			}

			if rec.Code == http.StatusCreated {
				var rule storage.ACLRule
				if err := json.NewDecoder(rec.Body).Decode(&rule); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if rule.TopicPattern != tt.request.TopicPattern {
					t.Errorf("CreateACL() topic = %v, want %v", rule.TopicPattern, tt.request.TopicPattern)
				}

				if rule.Permission != tt.request.Permission {
					t.Errorf("CreateACL() permission = %v, want %v", rule.Permission, tt.request.Permission)
				}

				if rule.ID == 0 {
					t.Errorf("CreateACL() ID should not be 0")
				}
			}
		})
	}
}

func TestCreateACL_InvalidJSON(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/acl", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.CreateACL(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateACL() with invalid JSON status = %v, want %v", rec.Code, http.StatusBadRequest)
	}
}

func TestDeleteACL(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test data
	mqttUser, err := handler.db.CreateMQTTUser("testuser", "password123", "Test user", nil)
	if err != nil {
		t.Fatalf("Failed to create test MQTT user: %v", err)
	}

	rule, err := handler.db.CreateACLRule(int(mqttUser.ID), "sensor/#", "pubsub")
	if err != nil {
		t.Fatalf("Failed to create test ACL rule: %v", err)
	}

	tests := []struct {
		name           string
		id             string
		wantStatusCode int
	}{
		{
			name:           "delete existing rule",
			id:             fmt.Sprintf("%d", rule.ID),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "delete non-existent rule",
			id:             "999999",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "delete with invalid ID",
			id:             "invalid",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/acl/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.DeleteACL(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteACL() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response body: %s", rec.Body.String())
			}
		})
	}
}

func TestListClients(t *testing.T) {
	t.Skip("Requires MQTT server implementation - skipping for unit tests")
	handler := setupTestHandler(t)

	// Create test data
	mqttUser, err := handler.db.CreateMQTTUser("testuser", "password123", "Test user", nil)
	if err != nil {
		t.Fatalf("Failed to create test MQTT user: %v", err)
	}

	client1, err := handler.db.UpsertMQTTClient("device-001", mqttUser.ID, nil)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	client2, err := handler.db.UpsertMQTTClient("device-002", mqttUser.ID, nil)
	if err != nil {
		t.Fatalf("Failed to create second test client: %v", err)
	}

	// Mark one inactive
	handler.db.MarkMQTTClientInactive(client1.ClientID)

	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	rec := httptest.NewRecorder()

	handler.ListClients(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListClients() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var clients []storage.MQTTClient
	if err := json.NewDecoder(rec.Body).Decode(&clients); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return all clients (active and inactive) by default
	if len(clients) != 2 {
		t.Errorf("ListClients() returned %d clients, want 2", len(clients))
	}

	// Verify client IDs
	foundClient1 := false
	foundClient2 := false
	for _, client := range clients {
		if client.ClientID == client1.ClientID {
			foundClient1 = true
		}
		if client.ClientID == client2.ClientID {
			foundClient2 = true
		}
	}

	if !foundClient1 {
		t.Errorf("ListClients() did not return client1")
	}
	if !foundClient2 {
		t.Errorf("ListClients() did not return client2")
	}
}

func TestListClients_Empty(t *testing.T) {
	t.Skip("Requires MQTT server implementation - skipping for unit tests")
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	rec := httptest.NewRecorder()

	handler.ListClients(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListClients() empty status = %v, want %v", rec.Code, http.StatusOK)
	}

	var clients []storage.MQTTClient
	if err := json.NewDecoder(rec.Body).Decode(&clients); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if clients == nil {
		t.Errorf("ListClients() returned nil, should return empty array")
	}

	if len(clients) != 0 {
		t.Errorf("ListClients() empty returned %d clients, want 0", len(clients))
	}
}

func TestGetClientDetails(t *testing.T) {
	t.Skip("Requires MQTT server implementation - skipping for unit tests")
	handler := setupTestHandler(t)

	// Create test data
	mqttUser, err := handler.db.CreateMQTTUser("testuser", "password123", "Test user", nil)
	if err != nil {
		t.Fatalf("Failed to create test MQTT user: %v", err)
	}

	client, err := handler.db.UpsertMQTTClient("device-001", mqttUser.ID, nil)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	tests := []struct {
		name           string
		id             string
		wantStatusCode int
	}{
		{
			name:           "get existing client",
			id:             fmt.Sprintf("%d", client.ID),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "get non-existent client",
			id:             "999999",
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "get with invalid ID",
			id:             "invalid",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/clients/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.GetClientDetails(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("GetClientDetails() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response body: %s", rec.Body.String())
			}

			if rec.Code == http.StatusOK {
				var returnedClient storage.MQTTClient
				if err := json.NewDecoder(rec.Body).Decode(&returnedClient); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if returnedClient.ClientID != client.ClientID {
					t.Errorf("GetClientDetails() clientID = %v, want %v", returnedClient.ClientID, client.ClientID)
				}
			}
		})
	}
}

func TestGetMetrics(t *testing.T) {
	t.Skip("Requires MQTT server implementation - skipping for unit tests")
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	rec := httptest.NewRecorder()

	handler.GetMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetMetrics() status = %v, want %v", rec.Code, http.StatusOK)
	}

	// Verify response is valid JSON
	var metrics map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&metrics); err != nil {
		t.Fatalf("Failed to decode metrics response: %v", err)
	}

	// Note: With a nil MQTT server, metrics may be empty or have default values
	// This test mainly verifies the endpoint doesn't crash
}

func TestHandlerCRUD_ACL_Integration(t *testing.T) {
	handler := setupTestHandler(t)

	// Create MQTT user
	mqttUser, err := handler.db.CreateMQTTUser("testuser", "password123", "Test user", nil)
	if err != nil {
		t.Fatalf("Failed to create test MQTT user: %v", err)
	}

	// 1. List ACL rules (should be empty)
	req := httptest.NewRequest(http.MethodGet, "/api/acl", nil)
	rec := httptest.NewRecorder()
	handler.ListACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Initial ListACL() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response1 struct {
		Data       []storage.ACLRule  `json:"data"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	json.NewDecoder(rec.Body).Decode(&response1)
	if len(response1.Data) != 0 {
		t.Fatalf("Initial ListACL() returned %d rules, want 0", len(response1.Data))
	}

	// 2. Create ACL rule
	createReq := CreateACLRequest{
		MQTTUserID:   int(mqttUser.ID),
		TopicPattern: "sensor/#",
		Permission:   "pubsub",
	}
	body, _ := json.Marshal(createReq)
	req = httptest.NewRequest(http.MethodPost, "/api/acl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.CreateACL(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateACL() status = %v, want %v", rec.Code, http.StatusCreated)
	}

	var createdRule storage.ACLRule
	json.NewDecoder(rec.Body).Decode(&createdRule)

	// 3. List ACL rules (should have 1)
	req = httptest.NewRequest(http.MethodGet, "/api/acl", nil)
	rec = httptest.NewRecorder()
	handler.ListACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListACL() after create status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response2 struct {
		Data       []storage.ACLRule  `json:"data"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	json.NewDecoder(rec.Body).Decode(&response2)
	if len(response2.Data) != 1 {
		t.Fatalf("ListACL() after create returned %d rules, want 1", len(response2.Data))
	}

	// 4. Delete ACL rule
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/acl/%d", createdRule.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", createdRule.ID))
	rec = httptest.NewRecorder()
	handler.DeleteACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("DeleteACL() status = %v, want %v", rec.Code, http.StatusOK)
	}

	// 5. List ACL rules (should be empty again)
	req = httptest.NewRequest(http.MethodGet, "/api/acl", nil)
	rec = httptest.NewRecorder()
	handler.ListACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Final ListACL() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response3 struct {
		Data       []storage.ACLRule  `json:"data"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	json.NewDecoder(rec.Body).Decode(&response3)
	if len(response3.Data) != 0 {
		t.Fatalf("Final ListACL() returned %d rules, want 0", len(response3.Data))
	}
}
