package storage

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name     string
		username string
		password string
		role     string
		wantErr  bool
	}{
		{
			name:     "create valid user",
			username: "testuser",
			password: "password123",
			role:     "user",
			wantErr:  false,
		},
		{
			name:     "create valid admin",
			username: "testadmin",
			password: "password123",
			role:     "admin",
			wantErr:  false,
		},
		{
			name:     "create user with invalid role",
			username: "invalidrole",
			password: "password123",
			role:     "superadmin",
			wantErr:  true,
		},
		{
			name:     "create duplicate username",
			username: "admin", // default admin already exists
			password: "password123",
			role:     "user",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.CreateUser(tt.username, tt.password, tt.role)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateUser() unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("CreateUser() username = %v, want %v", user.Username, tt.username)
			}

			if user.Role != tt.role {
				t.Errorf("CreateUser() role = %v, want %v", user.Role, tt.role)
			}

			if user.ID == 0 {
				t.Errorf("CreateUser() ID should not be 0")
			}

			// Verify password is hashed
			if user.PasswordHash == tt.password {
				t.Errorf("CreateUser() password should be hashed")
			}

			// Verify password hash is valid
			err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.password))
			if err != nil {
				t.Errorf("CreateUser() password hash is invalid: %v", err)
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test user
	created := createTestUser(t, db, "testuser", "password123", "user")

	tests := []struct {
		name    string
		id      int
		wantNil bool
		wantErr bool
	}{
		{
			name:    "get existing user",
			id:      created.ID,
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "get non-existent user",
			id:      999999,
			wantNil: true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.GetUser(tt.id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetUser() unexpected error: %v", err)
			}

			if tt.wantNil {
				if user != nil {
					t.Errorf("GetUser() expected nil but got user")
				}
				return
			}

			if user == nil {
				t.Fatalf("GetUser() expected user but got nil")
			}

			if user.ID != tt.id {
				t.Errorf("GetUser() ID = %v, want %v", user.ID, tt.id)
			}
		})
	}
}

func TestGetUserByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test user
	createTestUser(t, db, "testuser", "password123", "user")

	tests := []struct {
		name     string
		username string
		wantNil  bool
		wantErr  bool
	}{
		{
			name:     "get existing user",
			username: "testuser",
			wantNil:  false,
			wantErr:  false,
		},
		{
			name:     "get default admin",
			username: "admin",
			wantNil:  false,
			wantErr:  false,
		},
		{
			name:     "get non-existent user",
			username: "nonexistent",
			wantNil:  true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.GetUserByUsername(tt.username)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetUserByUsername() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetUserByUsername() unexpected error: %v", err)
			}

			if tt.wantNil {
				if user != nil {
					t.Errorf("GetUserByUsername() expected nil but got user")
				}
				return
			}

			if user == nil {
				t.Fatalf("GetUserByUsername() expected user but got nil")
			}

			if user.Username != tt.username {
				t.Errorf("GetUserByUsername() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test users
	createTestUser(t, db, "user1", "password123", "user")
	createTestUser(t, db, "user2", "password123", "user")
	createTestUser(t, db, "user3", "password123", "admin")

	users, err := db.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() unexpected error: %v", err)
	}

	// Should have 4 users (3 created + 1 default admin)
	if len(users) != 4 {
		t.Errorf("ListUsers() returned %d users, want 4", len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test user
	user := createTestUser(t, db, "testuser", "password123", "user")

	tests := []struct {
		name        string
		id          int
		newUsername string
		newRole     string
		wantErr     bool
	}{
		{
			name:        "update username",
			id:          user.ID,
			newUsername: "updateduser",
			newRole:     "user",
			wantErr:     false,
		},
		{
			name:        "update role to admin",
			id:          user.ID,
			newUsername: "updateduser",
			newRole:     "admin",
			wantErr:     false,
		},
		{
			name:        "update with invalid role",
			id:          user.ID,
			newUsername: "updateduser",
			newRole:     "superadmin",
			wantErr:     true,
		},
		{
			name:        "update non-existent user",
			id:          999999,
			newUsername: "ghost",
			newRole:     "user",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.UpdateUser(tt.id, tt.newUsername, tt.newRole)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateUser() unexpected error: %v", err)
			}

			// Verify the update
			updated, err := db.GetUser(tt.id)
			if err != nil {
				t.Fatalf("GetUser() after update failed: %v", err)
			}

			if updated.Username != tt.newUsername {
				t.Errorf("UpdateUser() username = %v, want %v", updated.Username, tt.newUsername)
			}

			if updated.Role != tt.newRole {
				t.Errorf("UpdateUser() role = %v, want %v", updated.Role, tt.newRole)
			}
		})
	}
}

func TestUpdateUserPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test user
	user := createTestUser(t, db, "testuser", "oldpassword", "user")

	tests := []struct {
		name        string
		id          int
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
			err := db.UpdateUserPassword(tt.id, tt.newPassword)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateUserPassword() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateUserPassword() unexpected error: %v", err)
			}

			// Verify the password was updated
			updated, err := db.GetUser(tt.id)
			if err != nil {
				t.Fatalf("GetUser() after password update failed: %v", err)
			}

			// Check if new password is correct
			err = bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte(tt.newPassword))
			if err != nil {
				t.Errorf("UpdateUserPassword() new password verification failed: %v", err)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		setup   func() int // returns user ID to delete
		wantErr bool
	}{
		{
			name: "delete existing user",
			setup: func() int {
				user := createTestUser(t, db, "todelete", "password123", "user")
				return user.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existent user",
			setup: func() int {
				return 999999
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := db.DeleteUser(id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteUser() unexpected error: %v", err)
			}

			// Verify user is deleted
			user, err := db.GetUser(id)
			if err != nil {
				t.Fatalf("GetUser() after delete failed: %v", err)
			}

			if user != nil {
				t.Errorf("DeleteUser() user still exists after deletion")
			}
		})
	}
}

func TestAuthenticateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test user
	createTestUser(t, db, "testuser", "correctpassword", "user")

	tests := []struct {
		name     string
		username string
		password string
		wantNil  bool
		wantErr  bool
	}{
		{
			name:     "authenticate with correct credentials",
			username: "testuser",
			password: "correctpassword",
			wantNil:  false,
			wantErr:  false,
		},
		{
			name:     "authenticate default admin",
			username: "admin",
			password: "admin",
			wantNil:  false,
			wantErr:  false,
		},
		{
			name:     "authenticate with wrong password",
			username: "testuser",
			password: "wrongpassword",
			wantNil:  true,
			wantErr:  false,
		},
		{
			name:     "authenticate non-existent user",
			username: "nonexistent",
			password: "password123",
			wantNil:  true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := db.AuthenticateUser(tt.username, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AuthenticateUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("AuthenticateUser() unexpected error: %v", err)
			}

			if tt.wantNil {
				if result != nil {
					t.Errorf("AuthenticateUser() expected nil but got result")
				}
				return
			}

			if result == nil {
				t.Fatalf("AuthenticateUser() expected result but got nil")
			}

			user, ok := result.(*User)
			if !ok {
				t.Fatalf("AuthenticateUser() result is not *User type")
			}

			if user.Username != tt.username {
				t.Errorf("AuthenticateUser() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}
