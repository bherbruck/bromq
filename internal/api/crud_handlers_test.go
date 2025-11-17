package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github/bherbruck/bromq/internal/storage"
	"gorm.io/datatypes"
)

// Test JWT secret for tests
var testJWTSecret = []byte("test-secret-key-for-unit-tests")

// Helper function to add admin token to request
func addAdminAuth(t *testing.T, req *http.Request) {
	token, err := GenerateJWT(testJWTSecret, 1, "admin", "admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
}

// Helper function to add user token to request
func addUserAuth(t *testing.T, req *http.Request) {
	token, err := GenerateJWT(testJWTSecret, 2, "user", "user")
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
}

// Helper to add auth claims to context directly (for handlers that expect middleware)
func addAdminToContext(req *http.Request) *http.Request {
	claims := &JWTClaims{
		UserID:   1,
		Username: "admin",
		Role:     "admin",
	}
	ctx := context.WithValue(req.Context(), userContextKey, claims)
	return req.WithContext(ctx)
}

func addUserToContext(req *http.Request) *http.Request {
	claims := &JWTClaims{
		UserID:   2,
		Username: "user",
		Role:     "user",
	}
	ctx := context.WithValue(req.Context(), userContextKey, claims)
	return req.WithContext(ctx)
}

// ==================== DashboardUser Management Tests ====================

func TestListDashboardUsers(t *testing.T) {
	handler := setupTestHandler(t)

	// Create some test admin users
	handler.db.CreateDashboardUser("admin1", "password123", "admin")
	handler.db.CreateDashboardUser("admin2", "password123", "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	handler.ListDashboardUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListDashboardUsers() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response struct {
		Data       []storage.DashboardUser `json:"data"`
		Pagination PaginationMetadata      `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have at least 3 users (default admin + 2 created)
	if len(response.Data) < 3 {
		t.Errorf("ListDashboardUsers() returned %d users, want at least 3", len(response.Data))
	}
}

func TestCreateDashboardUser(t *testing.T) {
	handler := setupTestHandler(t)

	tests := []struct {
		name           string
		request        CreateDashboardUserRequest
		wantStatusCode int
	}{
		{
			name: "create valid admin",
			request: CreateDashboardUserRequest{
				Username: "newadmin",
				Password: "password123",
				Role:     "admin",
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "create with invalid role",
			request: CreateDashboardUserRequest{
				Username: "invalidrole",
				Password: "password123",
				Role:     "superadmin",
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "create duplicate username",
			request: CreateDashboardUserRequest{
				Username: "admin", // default admin already exists
				Password: "password123",
				Role:     "admin",
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.CreateDashboardUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("CreateDashboardUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if rec.Code == http.StatusCreated {
				var user storage.DashboardUser
				if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if user.Username != tt.request.Username {
					t.Errorf("CreateDashboardUser() username = %v, want %v", user.Username, tt.request.Username)
				}

				if user.Role != tt.request.Role {
					t.Errorf("CreateDashboardUser() role = %v, want %v", user.Role, tt.request.Role)
				}
			}
		})
	}
}

func TestUpdateDashboardUser(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test user
	user, _ := handler.db.CreateDashboardUser("testadmin", "password123", "admin")

	tests := []struct {
		name           string
		id             string
		request        UpdateDashboardUserRequest
		wantStatusCode int
	}{
		{
			name: "update valid user",
			id:   fmt.Sprintf("%d", user.ID),
			request: UpdateDashboardUserRequest{
				Username: "updatedadmin",
				Role:     "admin",
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "update non-existent user",
			id:   "999999",
			request: UpdateDashboardUserRequest{
				Username: "ghost",
				Role:     "admin",
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "update with invalid ID",
			id:   "invalid",
			request: UpdateDashboardUserRequest{
				Username: "test",
				Role:     "admin",
			},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+tt.id, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.UpdateDashboardUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateDashboardUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

func TestDeleteDashboardUser(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test user
	user, _ := handler.db.CreateDashboardUser("todelete", "password123", "admin")

	tests := []struct {
		name           string
		id             string
		addContext     bool
		wantStatusCode int
	}{
		{
			name:           "delete other user",
			id:             fmt.Sprintf("%d", user.ID),
			addContext:     true, // Add admin context
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "delete non-existent user",
			id:             "999999",
			addContext:     true,
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "delete with invalid ID",
			id:             "invalid",
			addContext:     true,
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+tt.id, nil)
			req.SetPathValue("id", tt.id)

			if tt.addContext {
				req = addAdminToContext(req)
			}

			rec := httptest.NewRecorder()

			handler.DeleteDashboardUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteDashboardUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

func TestDeleteDashboardUser_PreventSelfDelete(t *testing.T) {
	handler := setupTestHandler(t)

	// Try to delete yourself (user ID 1 in context)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/1", nil)
	req.SetPathValue("id", "1")
	req = addAdminToContext(req) // Admin with ID 1

	rec := httptest.NewRecorder()

	handler.DeleteDashboardUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("DeleteDashboardUser() self-delete status = %v, want %v", rec.Code, http.StatusBadRequest)
	}
}

func TestChangePassword(t *testing.T) {
	handler := setupTestHandler(t)

	tests := []struct {
		name           string
		request        ChangePasswordRequest
		wantStatusCode int
	}{
		{
			name: "change password successfully",
			request: ChangePasswordRequest{
				CurrentPassword: "admin",
				NewPassword:     "newpassword123",
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "change password with wrong current password",
			request: ChangePasswordRequest{
				CurrentPassword: "wrongpassword",
				NewPassword:     "newpassword123",
			},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/api/auth/change-password", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = addAdminToContext(req)

			rec := httptest.NewRecorder()

			handler.ChangePassword(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("ChangePassword() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

func TestChangePassword_NoAuth(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/auth/change-password", nil)
	rec := httptest.NewRecorder()

	handler.ChangePassword(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("ChangePassword() without auth status = %v, want %v", rec.Code, http.StatusUnauthorized)
	}
}

// ==================== MQTTUser Management Tests ====================

func TestListMQTTUsers(t *testing.T) {
	handler := setupTestHandler(t)

	// Create some test MQTT users
	handler.db.CreateMQTTUser("device1", "password123", "Device 1", nil)
	handler.db.CreateMQTTUser("device2", "password123", "Device 2", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/mqtt/users", nil)
	rec := httptest.NewRecorder()

	handler.ListMQTTUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListMQTTUsers() status = %v, want %v", rec.Code, http.StatusOK)
	}

	var response struct {
		Data       []storage.MQTTUser `json:"data"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("ListMQTTUsers() returned %d users, want 2", len(response.Data))
	}
}

func TestCreateMQTTUser(t *testing.T) {
	handler := setupTestHandler(t)

	tests := []struct {
		name           string
		request        CreateMQTTUserRequest
		wantStatusCode int
	}{
		{
			name: "create valid MQTT user",
			request: CreateMQTTUserRequest{
				Username:    "device001",
				Password:    "password123",
				Description: "Test device",
				Metadata:    datatypes.JSON([]byte(`{"type":"sensor"}`)),
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "create with empty description",
			request: CreateMQTTUserRequest{
				Username: "device002",
				Password: "password123",
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "create with duplicate username",
			request: CreateMQTTUserRequest{
				Username: "device001", // Already created above
				Password: "password123",
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/mqtt/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.CreateMQTTUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("CreateMQTTUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if rec.Code == http.StatusCreated {
				var user storage.MQTTUser
				if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if user.Username != tt.request.Username {
					t.Errorf("CreateMQTTUser() username = %v, want %v", user.Username, tt.request.Username)
				}
			}
		})
	}
}

func TestUpdateMQTTUser(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test user
	user, _ := handler.db.CreateMQTTUser("devicetest", "password123", "Test", nil)

	tests := []struct {
		name           string
		id             string
		request        UpdateMQTTUserRequest
		wantStatusCode int
	}{
		{
			name: "update valid user",
			id:   fmt.Sprintf("%d", user.ID),
			request: UpdateMQTTUserRequest{
				Username:    "deviceupdated",
				Description: "Updated description",
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "update non-existent user",
			id:   "999999",
			request: UpdateMQTTUserRequest{
				Username: "ghost",
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "update with invalid ID",
			id:   "invalid",
			request: UpdateMQTTUserRequest{
				Username: "test",
			},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/api/mqtt/users/"+tt.id, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.UpdateMQTTUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateMQTTUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

func TestUpdateMQTTUserPassword(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test user
	user, _ := handler.db.CreateMQTTUser("devicepw", "oldpassword", "Test", nil)

	tests := []struct {
		name           string
		id             string
		request        UpdateMQTTPasswordRequest
		wantStatusCode int
	}{
		{
			name: "update password successfully",
			id:   fmt.Sprintf("%d", user.ID),
			request: UpdateMQTTPasswordRequest{
				Password: "newpassword123",
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "update non-existent user password",
			id:   "999999",
			request: UpdateMQTTPasswordRequest{
				Password: "password",
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "update with invalid ID",
			id:   "invalid",
			request: UpdateMQTTPasswordRequest{
				Password: "password",
			},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/api/mqtt/users/"+tt.id+"/password", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.UpdateMQTTUserPassword(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateMQTTUserPassword() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

func TestDeleteMQTTUser(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test user
	user, _ := handler.db.CreateMQTTUser("devicedelete", "password123", "To delete", nil)

	tests := []struct {
		name           string
		id             string
		wantStatusCode int
	}{
		{
			name:           "delete existing user",
			id:             fmt.Sprintf("%d", user.ID),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "delete non-existent user",
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
			req := httptest.NewRequest(http.MethodDelete, "/api/mqtt/users/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.DeleteMQTTUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteMQTTUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

// ==================== MQTTClient Management Tests ====================

func TestListMQTTClients(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test MQTT user and clients
	mqttUser, _ := handler.db.CreateMQTTUser("testdevice", "password123", "Test", nil)
	handler.db.UpsertMQTTClient("device-001", mqttUser.ID, nil)
	handler.db.UpsertMQTTClient("device-002", mqttUser.ID, nil)
	client3, _ := handler.db.UpsertMQTTClient("device-003", mqttUser.ID, nil)
	handler.db.MarkMQTTClientInactive(client3.ClientID)

	tests := []struct {
		name       string
		queryParam string
		wantCount  int
	}{
		{
			name:       "list all clients",
			queryParam: "",
			wantCount:  3,
		},
		{
			name:       "list active clients only",
			queryParam: "?active=true",
			wantCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/mqtt/clients"+tt.queryParam, nil)
			rec := httptest.NewRecorder()

			handler.ListMQTTClients(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ListMQTTClients() status = %v, want %v", rec.Code, http.StatusOK)
			}

			var response struct {
				Data       []storage.MQTTClient `json:"data"`
				Pagination PaginationMetadata   `json:"pagination"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(response.Data) != tt.wantCount {
				t.Errorf("ListMQTTClients() returned %d clients, want %d", len(response.Data), tt.wantCount)
			}
		})
	}
}

func TestGetMQTTClientDetails(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test MQTT user and client
	mqttUser, _ := handler.db.CreateMQTTUser("testdevice", "password123", "Test", nil)
	client, _ := handler.db.UpsertMQTTClient("device-details", mqttUser.ID, nil)

	tests := []struct {
		name           string
		clientID       string
		wantStatusCode int
	}{
		{
			name:           "get existing client",
			clientID:       client.ClientID,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "get non-existent client",
			clientID:       "nonexistent",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "get with empty client ID",
			clientID:       "",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/mqtt/clients/"+tt.clientID, nil)
			req.SetPathValue("client_id", tt.clientID)
			rec := httptest.NewRecorder()

			handler.GetMQTTClientDetails(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("GetMQTTClientDetails() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if rec.Code == http.StatusOK {
				var returnedClient storage.MQTTClient
				if err := json.NewDecoder(rec.Body).Decode(&returnedClient); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if returnedClient.ClientID != tt.clientID {
					t.Errorf("GetMQTTClientDetails() clientID = %v, want %v", returnedClient.ClientID, tt.clientID)
				}
			}
		})
	}
}

func TestUpdateMQTTClientMetadata(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test MQTT user and client
	mqttUser, _ := handler.db.CreateMQTTUser("testdevice", "password123", "Test", nil)
	client, _ := handler.db.UpsertMQTTClient("device-metadata", mqttUser.ID, nil)

	tests := []struct {
		name           string
		clientID       string
		request        UpdateMQTTClientMetadataRequest
		wantStatusCode int
	}{
		{
			name:     "update metadata successfully",
			clientID: client.ClientID,
			request: UpdateMQTTClientMetadataRequest{
				Metadata: datatypes.JSON([]byte(`{"location":"kitchen","type":"sensor"}`)),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:     "update non-existent client",
			clientID: "nonexistent",
			request: UpdateMQTTClientMetadataRequest{
				Metadata: datatypes.JSON([]byte(`{"test":true}`)),
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "update with empty client ID",
			clientID:       "",
			request:        UpdateMQTTClientMetadataRequest{},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/api/mqtt/clients/"+tt.clientID+"/metadata", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("client_id", tt.clientID)
			rec := httptest.NewRecorder()

			handler.UpdateMQTTClientMetadata(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateMQTTClientMetadata() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

func TestDeleteMQTTClient(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test MQTT user and client
	mqttUser, _ := handler.db.CreateMQTTUser("testdevice", "password123", "Test", nil)
	client, _ := handler.db.UpsertMQTTClient("device-delete", mqttUser.ID, nil)

	tests := []struct {
		name           string
		id             string
		wantStatusCode int
	}{
		{
			name:           "delete existing client",
			id:             fmt.Sprintf("%d", client.ID),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "delete non-existent client",
			id:             "999999",
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name:           "delete with invalid ID",
			id:             "invalid",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/mqtt/clients/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.DeleteMQTTClient(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteMQTTClient() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}
		})
	}
}

// ==================== Missing Tests ====================

func TestUpdateDashboardUserPassword(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test user
	user, _ := handler.db.CreateDashboardUser("testadminpw", "oldpassword", "admin")

	tests := []struct {
		name           string
		id             string
		request        UpdateAdminPasswordRequest
		wantStatusCode int
	}{
		{
			name: "update password successfully",
			id:   fmt.Sprintf("%d", user.ID),
			request: UpdateAdminPasswordRequest{
				Password: "newpassword123",
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "update non-existent user password",
			id:   "999999",
			request: UpdateAdminPasswordRequest{
				Password: "password",
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "update with invalid ID",
			id:   "invalid",
			request: UpdateAdminPasswordRequest{
				Password: "password",
			},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+tt.id+"/password", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			handler.UpdateDashboardUserPassword(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateDashboardUserPassword() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			// Verify password was actually changed
			if rec.Code == http.StatusOK {
				// Try to authenticate with new password
				user, err := handler.db.AuthenticateDashboardUser("testadminpw", "newpassword123")
				if err != nil || user == nil {
					t.Errorf("UpdateDashboardUserPassword() password not updated correctly")
				}
			}
		})
	}
}

func TestUpdateDashboardUserPassword_InvalidJSON(t *testing.T) {
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1/password", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.UpdateDashboardUserPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("UpdateDashboardUserPassword() invalid JSON status = %v, want %v", rec.Code, http.StatusBadRequest)
	}
}
// ==================== Provisioned Item Protection Tests ====================

func TestBlockProvisionedMQTTUserUpdate(t *testing.T) {
	handler := setupTestHandler(t)

	// Create a manual user and a provisioned user
	manualUser, _ := handler.db.CreateMQTTUser("manual_user", "password123", "Manual user", nil)
	provisionedUser, _ := handler.db.CreateMQTTUser("provisioned_user", "password123", "Provisioned user", nil)
	handler.db.MarkAsProvisioned(provisionedUser.ID, true)

	tests := []struct {
		name           string
		userID         uint
		wantStatusCode int
		wantError      string
	}{
		{
			name:           "update manual user succeeds",
			userID:         manualUser.ID,
			wantStatusCode: http.StatusOK,
			wantError:      "",
		},
		{
			name:           "update provisioned user blocked",
			userID:         provisionedUser.ID,
			wantStatusCode: http.StatusConflict,
			wantError:      "Cannot modify provisioned user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(UpdateMQTTUserRequest{
				Username:    "updated_name",
				Description: "Updated description",
			})
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/mqtt/users/%d", tt.userID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", fmt.Sprintf("%d", tt.userID))
			rec := httptest.NewRecorder()

			handler.UpdateMQTTUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateMQTTUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if tt.wantError != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantError)) {
				t.Errorf("Expected error message containing '%s', got: %s", tt.wantError, rec.Body.String())
			}
		})
	}
}

func TestBlockProvisionedMQTTUserDelete(t *testing.T) {
	handler := setupTestHandler(t)

	// Create a manual user and a provisioned user
	manualUser, _ := handler.db.CreateMQTTUser("manual_user_del", "password123", "Manual user", nil)
	provisionedUser, _ := handler.db.CreateMQTTUser("provisioned_user_del", "password123", "Provisioned user", nil)
	handler.db.MarkAsProvisioned(provisionedUser.ID, true)

	tests := []struct {
		name           string
		userID         uint
		wantStatusCode int
		wantError      string
	}{
		{
			name:           "delete manual user succeeds",
			userID:         manualUser.ID,
			wantStatusCode: http.StatusOK,
			wantError:      "",
		},
		{
			name:           "delete provisioned user blocked",
			userID:         provisionedUser.ID,
			wantStatusCode: http.StatusConflict,
			wantError:      "Cannot delete provisioned user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/mqtt/users/%d", tt.userID), nil)
			req.SetPathValue("id", fmt.Sprintf("%d", tt.userID))
			rec := httptest.NewRecorder()

			handler.DeleteMQTTUser(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteMQTTUser() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if tt.wantError != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantError)) {
				t.Errorf("Expected error message containing '%s', got: %s", tt.wantError, rec.Body.String())
			}
		})
	}
}

func TestBlockProvisionedMQTTUserPasswordUpdate(t *testing.T) {
	handler := setupTestHandler(t)

	// Create a manual user and a provisioned user
	manualUser, _ := handler.db.CreateMQTTUser("manual_user_pw", "password123", "Manual user", nil)
	provisionedUser, _ := handler.db.CreateMQTTUser("provisioned_user_pw", "password123", "Provisioned user", nil)
	handler.db.MarkAsProvisioned(provisionedUser.ID, true)

	tests := []struct {
		name           string
		userID         uint
		wantStatusCode int
		wantError      string
	}{
		{
			name:           "update manual user password succeeds",
			userID:         manualUser.ID,
			wantStatusCode: http.StatusOK,
			wantError:      "",
		},
		{
			name:           "update provisioned user password blocked",
			userID:         provisionedUser.ID,
			wantStatusCode: http.StatusConflict,
			wantError:      "Cannot modify provisioned user password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(UpdateMQTTPasswordRequest{
				Password: "newpassword123",
			})
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/mqtt/users/%d/password", tt.userID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", fmt.Sprintf("%d", tt.userID))
			rec := httptest.NewRecorder()

			handler.UpdateMQTTUserPassword(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateMQTTUserPassword() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if tt.wantError != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantError)) {
				t.Errorf("Expected error message containing '%s', got: %s", tt.wantError, rec.Body.String())
			}
		})
	}
}

func TestBlockProvisionedACLRuleUpdate(t *testing.T) {
	handler := setupTestHandler(t)

	// Create user and ACL rules
	user, _ := handler.db.CreateMQTTUser("acl_test_user", "password123", "Test user", nil)
	
	// Create manual rule
	manualRule, _ := handler.db.CreateACLRule(int(user.ID), "manual/topic/#", "pubsub")
	
	// Create provisioned rule
	handler.db.CreateProvisionedACLRule(user.ID, "provisioned/topic/#", "pubsub")
	provisionedRule, _ := handler.db.GetACLRulesByMQTTUserID(int(user.ID))
	var provisionedRuleID int
	for _, rule := range provisionedRule {
		if rule.ProvisionedFromConfig {
			provisionedRuleID = int(rule.ID)
			break
		}
	}

	tests := []struct {
		name           string
		ruleID         int
		wantStatusCode int
		wantError      string
	}{
		{
			name:           "update manual ACL rule succeeds",
			ruleID:         int(manualRule.ID),
			wantStatusCode: http.StatusOK,
			wantError:      "",
		},
		{
			name:           "update provisioned ACL rule blocked",
			ruleID:         provisionedRuleID,
			wantStatusCode: http.StatusConflict,
			wantError:      "Cannot modify provisioned ACL rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(UpdateACLRequest{
				Topic: "updated/topic/#",
				Permission:   "pub",
			})
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/acl/%d", tt.ruleID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", fmt.Sprintf("%d", tt.ruleID))
			rec := httptest.NewRecorder()

			handler.UpdateACL(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("UpdateACL() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if tt.wantError != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantError)) {
				t.Errorf("Expected error message containing '%s', got: %s", tt.wantError, rec.Body.String())
			}
		})
	}
}

func TestBlockProvisionedACLRuleDelete(t *testing.T) {
	handler := setupTestHandler(t)

	// Create user and ACL rules
	user, _ := handler.db.CreateMQTTUser("acl_del_test_user", "password123", "Test user", nil)
	
	// Create manual rule
	manualRule, _ := handler.db.CreateACLRule(int(user.ID), "manual/delete/#", "pubsub")
	
	// Create provisioned rule
	handler.db.CreateProvisionedACLRule(user.ID, "provisioned/delete/#", "pubsub")
	provisionedRule, _ := handler.db.GetACLRulesByMQTTUserID(int(user.ID))
	var provisionedRuleID int
	for _, rule := range provisionedRule {
		if rule.ProvisionedFromConfig {
			provisionedRuleID = int(rule.ID)
			break
		}
	}

	tests := []struct {
		name           string
		ruleID         int
		wantStatusCode int
		wantError      string
	}{
		{
			name:           "delete manual ACL rule succeeds",
			ruleID:         int(manualRule.ID),
			wantStatusCode: http.StatusOK,
			wantError:      "",
		},
		{
			name:           "delete provisioned ACL rule blocked",
			ruleID:         provisionedRuleID,
			wantStatusCode: http.StatusConflict,
			wantError:      "Cannot delete provisioned ACL rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/acl/%d", tt.ruleID), nil)
			req.SetPathValue("id", fmt.Sprintf("%d", tt.ruleID))
			rec := httptest.NewRecorder()

			handler.DeleteACL(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("DeleteACL() status = %v, want %v", rec.Code, tt.wantStatusCode)
				t.Logf("Response: %s", rec.Body.String())
			}

			if tt.wantError != "" && !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantError)) {
				t.Errorf("Expected error message containing '%s', got: %s", tt.wantError, rec.Body.String())
			}
		})
	}
}
