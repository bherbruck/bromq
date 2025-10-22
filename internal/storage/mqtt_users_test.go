package storage

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCreateMQTTUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name        string
		username    string
		password    string
		description string
		wantErr     bool
	}{
		{
			name:        "create valid MQTT user",
			username:    "sensor_user",
			password:    "password123",
			description: "Sensor credentials",
			wantErr:     false,
		},
		{
			name:        "create with empty description",
			username:    "device_user",
			password:    "password123",
			description: "",
			wantErr:     false,
		},
		{
			name:        "create with empty username",
			username:    "",
			password:    "password123",
			description: "Test",
			wantErr:     true,
		},
		{
			name:        "create with empty password",
			username:    "testuser2",
			password:    "",
			description: "Test",
			wantErr:     true,
		},
		{
			name:        "create duplicate username",
			username:    "sensor_user",
			password:    "password123",
			description: "Duplicate",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.CreateMQTTUser(tt.username, tt.password, tt.description, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateMQTTUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateMQTTUser() unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("CreateMQTTUser() username = %v, want %v", user.Username, tt.username)
			}

			if user.Description != tt.description {
				t.Errorf("CreateMQTTUser() description = %v, want %v", user.Description, tt.description)
			}

			if user.ID == 0 {
				t.Errorf("CreateMQTTUser() ID should not be 0")
			}

			// Verify password is hashed
			if user.PasswordHash == tt.password {
				t.Errorf("CreateMQTTUser() password should be hashed")
			}

			// Verify password hash is valid
			err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.password))
			if err != nil {
				t.Errorf("CreateMQTTUser() password hash is invalid: %v", err)
			}
		})
	}
}

func TestGetMQTTUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user
	created := createTestMQTTUser(t, db, "testuser", "password123", "Test user")

	tests := []struct {
		name    string
		id      uint
		wantErr bool
	}{
		{
			name:    "get existing MQTT user",
			id:      created.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent MQTT user",
			id:      999999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.GetMQTTUser(int(tt.id))

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetMQTTUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetMQTTUser() unexpected error: %v", err)
			}

			if user.ID != tt.id {
				t.Errorf("GetMQTTUser() ID = %v, want %v", user.ID, tt.id)
			}
		})
	}
}

func TestGetMQTTUserByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user
	createTestMQTTUser(t, db, "testuser", "password123", "Test user")

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "get existing MQTT user",
			username: "testuser",
			wantErr:  false,
		},
		{
			name:     "get non-existent MQTT user",
			username: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.GetMQTTUserByUsername(tt.username)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetMQTTUserByUsername() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetMQTTUserByUsername() unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("GetMQTTUserByUsername() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}

func TestListMQTTUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test MQTT users
	createTestMQTTUser(t, db, "user1", "password123", "User 1")
	createTestMQTTUser(t, db, "user2", "password123", "User 2")
	createTestMQTTUser(t, db, "user3", "password123", "User 3")

	users, err := db.ListMQTTUsers()
	if err != nil {
		t.Fatalf("ListMQTTUsers() unexpected error: %v", err)
	}

	if len(users) != 3 {
		t.Errorf("ListMQTTUsers() returned %d users, want 3", len(users))
	}
}

func TestUpdateMQTTUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user
	user := createTestMQTTUser(t, db, "testuser", "password123", "Original description")

	tests := []struct {
		name           string
		id             uint
		newUsername    string
		newDescription string
		wantErr        bool
	}{
		{
			name:           "update username and description",
			id:             user.ID,
			newUsername:    "updateduser",
			newDescription: "Updated description",
			wantErr:        false,
		},
		{
			name:           "update non-existent user",
			id:             999999,
			newUsername:    "ghost",
			newDescription: "Ghost user",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.UpdateMQTTUser(int(tt.id), tt.newUsername, tt.newDescription, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateMQTTUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateMQTTUser() unexpected error: %v", err)
			}

			// Verify the update
			updated, err := db.GetMQTTUser(int(tt.id))
			if err != nil {
				t.Fatalf("GetMQTTUser() after update failed: %v", err)
			}

			if updated.Username != tt.newUsername {
				t.Errorf("UpdateMQTTUser() username = %v, want %v", updated.Username, tt.newUsername)
			}

			if updated.Description != tt.newDescription {
				t.Errorf("UpdateMQTTUser() description = %v, want %v", updated.Description, tt.newDescription)
			}
		})
	}
}

func TestUpdateMQTTUserPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user
	user := createTestMQTTUser(t, db, "testuser", "oldpassword", "Test user")

	tests := []struct {
		name        string
		id          uint
		newPassword string
		wantErr     bool
	}{
		{
			name:        "update password",
			id:          user.ID,
			newPassword: "newpassword123",
			wantErr:     false,
		},
		{
			name:        "update non-existent user password",
			id:          999999,
			newPassword: "password123",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.UpdateMQTTUserPassword(int(tt.id), tt.newPassword)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateMQTTUserPassword() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateMQTTUserPassword() unexpected error: %v", err)
			}

			// Verify the password was updated
			updated, err := db.GetMQTTUser(int(tt.id))
			if err != nil {
				t.Fatalf("GetMQTTUser() after password update failed: %v", err)
			}

			// Check if new password is correct
			err = bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte(tt.newPassword))
			if err != nil {
				t.Errorf("UpdateMQTTUserPassword() new password verification failed: %v", err)
			}
		})
	}
}

func TestDeleteMQTTUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		setup   func() uint // returns user ID to delete
		wantErr bool
	}{
		{
			name: "delete existing MQTT user",
			setup: func() uint {
				user := createTestMQTTUser(t, db, "todelete", "password123", "To delete")
				return user.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existent MQTT user",
			setup: func() uint {
				return 999999
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := db.DeleteMQTTUser(int(id))

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteMQTTUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteMQTTUser() unexpected error: %v", err)
			}

			// Verify user is deleted
			_, err = db.GetMQTTUser(int(id))
			if err == nil {
				t.Errorf("DeleteMQTTUser() user still exists after deletion")
			}
		})
	}
}

func TestAuthenticateMQTTUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user
	createTestMQTTUser(t, db, "testuser", "correctpassword", "Test user")

	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "authenticate with correct credentials",
			username: "testuser",
			password: "correctpassword",
			wantErr:  false,
		},
		{
			name:     "authenticate with wrong password",
			username: "testuser",
			password: "wrongpassword",
			wantErr:  true,
		},
		{
			name:     "authenticate non-existent user",
			username: "nonexistent",
			password: "password123",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.AuthenticateMQTTUser(tt.username, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AuthenticateMQTTUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("AuthenticateMQTTUser() unexpected error: %v", err)
			}

			if user == nil {
				t.Fatalf("AuthenticateMQTTUser() expected result but got nil")
			}

			if user.Username != tt.username {
				t.Errorf("AuthenticateMQTTUser() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}

func TestAuthenticateUser_Compatibility(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test MQTT user
	createTestMQTTUser(t, db, "testuser", "correctpassword", "Test user")

	// Test the compatibility wrapper
	result, err := db.AuthenticateUser("testuser", "correctpassword")
	if err != nil {
		t.Fatalf("AuthenticateUser() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("AuthenticateUser() expected result but got nil")
	}

	// Should return MQTTUser
	user, ok := result.(*MQTTUser)
	if !ok {
		t.Fatalf("AuthenticateUser() result is not *MQTTUser type")
	}

	if user.Username != "testuser" {
		t.Errorf("AuthenticateUser() username = %v, want testuser", user.Username)
	}

	// Test with wrong password
	result, err = db.AuthenticateUser("testuser", "wrongpassword")
	if err == nil {
		t.Error("AuthenticateUser() with wrong password should return error")
	}
}
