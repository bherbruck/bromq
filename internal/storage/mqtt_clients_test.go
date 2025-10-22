package storage

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"
)

func TestUpsertMQTTClient_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user to associate with
	mqttUser := createTestMQTTUser(t, db, "device_user", "password123", "Device credentials")

	tests := []struct {
		name       string
		clientID   string
		mqttUserID uint
		metadata   datatypes.JSON
		wantErr    bool
	}{
		{
			name:       "create new client",
			clientID:   "device-001",
			mqttUserID: mqttUser.ID,
			metadata:   nil,
			wantErr:    false,
		},
		{
			name:       "create client with metadata",
			clientID:   "device-002",
			mqttUserID: mqttUser.ID,
			metadata:   datatypes.JSON([]byte(`{"type":"sensor","location":"kitchen"}`)),
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := db.UpsertMQTTClient(tt.clientID, tt.mqttUserID, tt.metadata)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpsertMQTTClient() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpsertMQTTClient() unexpected error: %v", err)
			}

			if client.ClientID != tt.clientID {
				t.Errorf("ClientID = %v, want %v", client.ClientID, tt.clientID)
			}

			if client.MQTTUserID != tt.mqttUserID {
				t.Errorf("MQTTUserID = %v, want %v", client.MQTTUserID, tt.mqttUserID)
			}

			if !client.IsActive {
				t.Errorf("Expected new client to be active")
			}

			if client.FirstSeen.IsZero() {
				t.Errorf("FirstSeen should be set")
			}

			if client.LastSeen.IsZero() {
				t.Errorf("LastSeen should be set")
			}

			if client.ID == 0 {
				t.Errorf("ID should not be 0")
			}
		})
	}
}

func TestUpsertMQTTClient_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser1 := createTestMQTTUser(t, db, "user1", "password123", "User 1")
	mqttUser2 := createTestMQTTUser(t, db, "user2", "password123", "User 2")

	clientID := "device-001"

	// Create initial client
	client1, err := db.UpsertMQTTClient(clientID, mqttUser1.ID, nil)
	if err != nil {
		t.Fatalf("Failed to create initial client: %v", err)
	}

	initialID := client1.ID
	initialFirstSeen := client1.FirstSeen

	// Upsert again (should update, not create)
	client2, err := db.UpsertMQTTClient(clientID, mqttUser2.ID, datatypes.JSON([]byte(`{"updated":true}`)))
	if err != nil {
		t.Fatalf("Failed to upsert existing client: %v", err)
	}

	// Should have same ID (update, not create)
	if client2.ID != initialID {
		t.Errorf("ID changed from %v to %v, expected same ID", initialID, client2.ID)
	}

	// FirstSeen should not change
	if !client2.FirstSeen.Equal(initialFirstSeen) {
		t.Errorf("FirstSeen changed, should remain the same")
	}

	// MQTTUserID should be updated
	if client2.MQTTUserID != mqttUser2.ID {
		t.Errorf("MQTTUserID = %v, want %v", client2.MQTTUserID, mqttUser2.ID)
	}

	// LastSeen should be updated
	if client2.LastSeen.Before(initialFirstSeen) || client2.LastSeen.Equal(initialFirstSeen) {
		t.Errorf("LastSeen should be updated to a later time")
	}

	// Should still be active
	if !client2.IsActive {
		t.Errorf("Client should still be active after upsert")
	}

	// Verify only one record exists
	count, err := db.GetClientCount(false)
	if err != nil {
		t.Fatalf("Failed to get client count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 client record, got %d", count)
	}
}

func TestMarkMQTTClientInactive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")

	tests := []struct {
		name     string
		setup    func() string // Returns client ID
		wantErr  bool
		checkErr func(error) bool
	}{
		{
			name: "mark existing client inactive",
			setup: func() string {
				client, _ := db.UpsertMQTTClient("device-active", mqttUser.ID, nil)
				return client.ClientID
			},
			wantErr: false,
		},
		{
			name: "mark non-existent client",
			setup: func() string {
				return "nonexistent-client"
			},
			wantErr: false, // GORM doesn't error on 0 rows affected for Updates
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientID := tt.setup()
			err := db.MarkMQTTClientInactive(clientID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MarkMQTTClientInactive() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("MarkMQTTClientInactive() unexpected error: %v", err)
			}

			// If client exists, verify it's marked inactive
			client, err := db.GetMQTTClientByClientID(clientID)
			if err == nil {
				if client.IsActive {
					t.Errorf("Client should be marked inactive")
				}
			}
		})
	}
}

func TestGetMQTTClient(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")
	created, _ := db.UpsertMQTTClient("device-001", mqttUser.ID, nil)

	tests := []struct {
		name    string
		id      int
		wantErr bool
	}{
		{
			name:    "get existing client",
			id:      int(created.ID),
			wantErr: false,
		},
		{
			name:    "get non-existent client",
			id:      999999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := db.GetMQTTClient(tt.id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetMQTTClient() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetMQTTClient() unexpected error: %v", err)
			}

			if client.ID != created.ID {
				t.Errorf("ID = %v, want %v", client.ID, created.ID)
			}

			// Verify preload worked
			if client.MQTTUser.ID == 0 {
				t.Errorf("MQTTUser should be preloaded")
			}

			if client.MQTTUser.Username != "testuser" {
				t.Errorf("MQTTUser.Username = %v, want testuser", client.MQTTUser.Username)
			}
		})
	}
}

func TestGetMQTTClientByClientID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")
	db.UpsertMQTTClient("device-001", mqttUser.ID, nil)

	tests := []struct {
		name     string
		clientID string
		wantErr  bool
	}{
		{
			name:     "get existing client",
			clientID: "device-001",
			wantErr:  false,
		},
		{
			name:     "get non-existent client",
			clientID: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := db.GetMQTTClientByClientID(tt.clientID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetMQTTClientByClientID() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetMQTTClientByClientID() unexpected error: %v", err)
			}

			if client.ClientID != tt.clientID {
				t.Errorf("ClientID = %v, want %v", client.ClientID, tt.clientID)
			}

			// Verify preload worked
			if client.MQTTUser.ID == 0 {
				t.Errorf("MQTTUser should be preloaded")
			}
		})
	}
}

func TestListMQTTClients(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")

	// Create test clients
	client1, _ := db.UpsertMQTTClient("device-001", mqttUser.ID, nil)
	db.UpsertMQTTClient("device-002", mqttUser.ID, nil)
	db.UpsertMQTTClient("device-003", mqttUser.ID, nil)

	// Mark one inactive
	db.MarkMQTTClientInactive(client1.ClientID)

	tests := []struct {
		name       string
		activeOnly bool
		wantCount  int
	}{
		{
			name:       "list all clients",
			activeOnly: false,
			wantCount:  3,
		},
		{
			name:       "list active clients only",
			activeOnly: true,
			wantCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients, err := db.ListMQTTClients(tt.activeOnly)
			if err != nil {
				t.Fatalf("ListMQTTClients() unexpected error: %v", err)
			}

			if len(clients) != tt.wantCount {
				t.Errorf("ListMQTTClients() returned %d clients, want %d", len(clients), tt.wantCount)
			}

			// Verify preload worked
			for _, client := range clients {
				if client.MQTTUser.ID == 0 {
					t.Errorf("MQTTUser should be preloaded for client %s", client.ClientID)
				}
			}
		})
	}
}

func TestListMQTTClientsByUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user1 := createTestMQTTUser(t, db, "user1", "password123", "User 1")
	user2 := createTestMQTTUser(t, db, "user2", "password123", "User 2")

	// Create clients for user1
	client1, _ := db.UpsertMQTTClient("user1-device-001", user1.ID, nil)
	db.UpsertMQTTClient("user1-device-002", user1.ID, nil)

	// Create clients for user2
	db.UpsertMQTTClient("user2-device-001", user2.ID, nil)

	// Mark one user1 client inactive
	db.MarkMQTTClientInactive(client1.ClientID)

	tests := []struct {
		name       string
		mqttUserID uint
		activeOnly bool
		wantCount  int
	}{
		{
			name:       "list all clients for user1",
			mqttUserID: user1.ID,
			activeOnly: false,
			wantCount:  2,
		},
		{
			name:       "list active clients for user1",
			mqttUserID: user1.ID,
			activeOnly: true,
			wantCount:  1,
		},
		{
			name:       "list all clients for user2",
			mqttUserID: user2.ID,
			activeOnly: false,
			wantCount:  1,
		},
		{
			name:       "list clients for non-existent user",
			mqttUserID: 999999,
			activeOnly: false,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients, err := db.ListMQTTClientsByUser(tt.mqttUserID, tt.activeOnly)
			if err != nil {
				t.Fatalf("ListMQTTClientsByUser() unexpected error: %v", err)
			}

			if len(clients) != tt.wantCount {
				t.Errorf("ListMQTTClientsByUser() returned %d clients, want %d", len(clients), tt.wantCount)
			}

			// Verify all clients belong to the right user
			for _, client := range clients {
				if client.MQTTUserID != tt.mqttUserID {
					t.Errorf("Client %s has MQTTUserID %d, want %d", client.ClientID, client.MQTTUserID, tt.mqttUserID)
				}
			}
		})
	}
}

func TestUpdateMQTTClientMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")
	client, _ := db.UpsertMQTTClient("device-001", mqttUser.ID, nil)

	tests := []struct {
		name     string
		clientID string
		metadata datatypes.JSON
		wantErr  bool
	}{
		{
			name:     "update existing client metadata",
			clientID: client.ClientID,
			metadata: datatypes.JSON([]byte(`{"type":"temperature","location":"living room"}`)),
			wantErr:  false,
		},
		{
			name:     "update non-existent client",
			clientID: "nonexistent",
			metadata: datatypes.JSON([]byte(`{"test":true}`)),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.UpdateMQTTClientMetadata(tt.clientID, tt.metadata)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateMQTTClientMetadata() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateMQTTClientMetadata() unexpected error: %v", err)
			}

			// Verify metadata was updated
			updated, err := db.GetMQTTClientByClientID(tt.clientID)
			if err != nil {
				t.Fatalf("Failed to get updated client: %v", err)
			}

			if string(updated.Metadata) != string(tt.metadata) {
				t.Errorf("Metadata = %s, want %s", updated.Metadata, tt.metadata)
			}
		})
	}
}

func TestDeleteMQTTClient(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		setup   func() int // Returns client ID to delete
		wantErr bool
	}{
		{
			name: "delete existing client",
			setup: func() int {
				mqttUser := createTestMQTTUser(t, db, "deluser1", "password123", "Test")
				client, _ := db.UpsertMQTTClient("delete-me", mqttUser.ID, nil)
				return int(client.ID)
			},
			wantErr: false,
		},
		{
			name: "delete non-existent client",
			setup: func() int {
				return 999999
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := db.DeleteMQTTClient(id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteMQTTClient() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteMQTTClient() unexpected error: %v", err)
			}

			// Verify client is deleted
			_, err = db.GetMQTTClient(id)
			if err == nil {
				t.Errorf("DeleteMQTTClient() client still exists after deletion")
			}
		})
	}
}

func TestGetClientCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")

	// Create test clients
	client1, _ := db.UpsertMQTTClient("count-device-001", mqttUser.ID, nil)
	db.UpsertMQTTClient("count-device-002", mqttUser.ID, nil)
	db.UpsertMQTTClient("count-device-003", mqttUser.ID, nil)

	// Mark one inactive
	db.MarkMQTTClientInactive(client1.ClientID)

	tests := []struct {
		name       string
		activeOnly bool
		wantCount  int64
	}{
		{
			name:       "count all clients",
			activeOnly: false,
			wantCount:  3,
		},
		{
			name:       "count active clients only",
			activeOnly: true,
			wantCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := db.GetClientCount(tt.activeOnly)
			if err != nil {
				t.Fatalf("GetClientCount() unexpected error: %v", err)
			}

			if count != tt.wantCount {
				t.Errorf("GetClientCount() = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestUpsertMQTTClientInterface(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")

	tests := []struct {
		name       string
		clientID   string
		mqttUserID uint
		metadata   interface{}
		wantErr    bool
	}{
		{
			name:       "upsert with nil metadata",
			clientID:   "interface-001",
			mqttUserID: mqttUser.ID,
			metadata:   nil,
			wantErr:    false,
		},
		{
			name:       "upsert with JSON metadata",
			clientID:   "interface-002",
			mqttUserID: mqttUser.ID,
			metadata:   datatypes.JSON([]byte(`{"interface":true}`)),
			wantErr:    false,
		},
		{
			name:       "upsert with non-JSON metadata (should handle gracefully)",
			clientID:   "interface-003",
			mqttUserID: mqttUser.ID,
			metadata:   "not json",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := db.UpsertMQTTClientInterface(tt.clientID, tt.mqttUserID, tt.metadata)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpsertMQTTClientInterface() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpsertMQTTClientInterface() unexpected error: %v", err)
			}

			// Result should be an *MQTTClient
			client, ok := result.(*MQTTClient)
			if !ok {
				t.Fatalf("UpsertMQTTClientInterface() result is not *MQTTClient type")
			}

			if client.ClientID != tt.clientID {
				t.Errorf("ClientID = %v, want %v", client.ClientID, tt.clientID)
			}
		})
	}
}

func TestMQTTClientMetadataJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mqttUser := createTestMQTTUser(t, db, "testuser", "password123", "Test")

	// Create client with JSON metadata
	metadata := datatypes.JSON([]byte(`{"device":"sensor","version":"1.2.3","location":"garage"}`))
	client, err := db.UpsertMQTTClient("json-test", mqttUser.ID, metadata)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Retrieve and parse metadata
	retrieved, err := db.GetMQTTClient(int(client.ID))
	if err != nil {
		t.Fatalf("Failed to retrieve client: %v", err)
	}

	// Parse JSON
	var parsed map[string]string
	err = json.Unmarshal(retrieved.Metadata, &parsed)
	if err != nil {
		t.Fatalf("Failed to parse metadata JSON: %v", err)
	}

	// Verify values
	if parsed["device"] != "sensor" {
		t.Errorf("metadata.device = %v, want sensor", parsed["device"])
	}
	if parsed["version"] != "1.2.3" {
		t.Errorf("metadata.version = %v, want 1.2.3", parsed["version"])
	}
	if parsed["location"] != "garage" {
		t.Errorf("metadata.location = %v, want garage", parsed["location"])
	}
}
