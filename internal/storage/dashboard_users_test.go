package storage

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCreateDashboardUser(t *testing.T) {
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
			name:     "create valid admin",
			username: "testadmin",
			password: "password123",
			role:     "admin",
			wantErr:  false,
		},
		{
			name:     "create with invalid role",
			username: "invalidrole",
			password: "password123",
			role:     "superadmin",
			wantErr:  true,
		},
		{
			name:     "create duplicate username",
			username: "admin", // default admin already exists
			password: "password123",
			role:     "admin",
			wantErr:  true,
		},
		{
			name:     "create with empty username",
			username: "",
			password: "password123",
			role:     "admin",
			wantErr:  true,
		},
		{
			name:     "create with empty password",
			username: "testadmin2",
			password: "",
			role:     "admin",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.CreateDashboardUser(tt.username, tt.password, tt.role)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateDashboardUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateDashboardUser() unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("CreateDashboardUser() username = %v, want %v", user.Username, tt.username)
			}

			if user.Role != tt.role {
				t.Errorf("CreateDashboardUser() role = %v, want %v", user.Role, tt.role)
			}

			if user.ID == 0 {
				t.Errorf("CreateDashboardUser() ID should not be 0")
			}

			// Verify password is hashed
			if user.PasswordHash == tt.password {
				t.Errorf("CreateDashboardUser() password should be hashed")
			}

			// Verify password hash is valid
			err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.password))
			if err != nil {
				t.Errorf("CreateDashboardUser() password hash is invalid: %v", err)
			}
		})
	}
}

func TestGetDashboardUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test admin user
	created := createTestDashboardUser(t, db, "testadmin", "password123", "admin")

	tests := []struct {
		name    string
		id      uint
		wantErr bool
	}{
		{
			name:    "get existing admin",
			id:      created.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent admin",
			id:      999999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.GetDashboardUser(int(tt.id))

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetDashboardUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetDashboardUser() unexpected error: %v", err)
			}

			if user.ID != tt.id {
				t.Errorf("GetDashboardUser() ID = %v, want %v", user.ID, tt.id)
			}
		})
	}
}

func TestGetDashboardUserByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test admin user
	createTestDashboardUser(t, db, "testadmin", "password123", "admin")

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "get existing admin",
			username: "testadmin",
			wantErr:  false,
		},
		{
			name:     "get default admin",
			username: "admin",
			wantErr:  false,
		},
		{
			name:     "get non-existent admin",
			username: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.GetDashboardUserByUsername(tt.username)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetDashboardUserByUsername() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetDashboardUserByUsername() unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("GetDashboardUserByUsername() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}

func TestListDashboardUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test admin users
	createTestDashboardUser(t, db, "admin1", "password123", "admin")
	createTestDashboardUser(t, db, "admin2", "password123", "admin")
	createTestDashboardUser(t, db, "admin3", "password123", "admin")

	users, err := db.ListDashboardUsers()
	if err != nil {
		t.Fatalf("ListDashboardUsers() unexpected error: %v", err)
	}

	// Should have 4 users (3 created + 1 default admin)
	if len(users) != 4 {
		t.Errorf("ListDashboardUsers() returned %d users, want 4", len(users))
	}
}

func TestUpdateDashboardUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test admin user
	user := createTestDashboardUser(t, db, "testadmin", "password123", "admin")

	tests := []struct {
		name        string
		id          uint
		newUsername string
		newRole     string
		wantErr     bool
	}{
		{
			name:        "update username",
			id:          user.ID,
			newUsername: "updatedadmin",
			newRole:     "admin",
			wantErr:     false,
		},
		{
			name:        "update with invalid role",
			id:          user.ID,
			newUsername: "updatedadmin",
			newRole:     "superadmin",
			wantErr:     true,
		},
		{
			name:        "update non-existent user",
			id:          999999,
			newUsername: "ghost",
			newRole:     "admin",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.UpdateDashboardUser(int(tt.id), tt.newUsername, tt.newRole)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateDashboardUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateDashboardUser() unexpected error: %v", err)
			}

			// Verify the update
			updated, err := db.GetDashboardUser(int(tt.id))
			if err != nil {
				t.Fatalf("GetDashboardUser() after update failed: %v", err)
			}

			if updated.Username != tt.newUsername {
				t.Errorf("UpdateDashboardUser() username = %v, want %v", updated.Username, tt.newUsername)
			}

			if updated.Role != tt.newRole {
				t.Errorf("UpdateDashboardUser() role = %v, want %v", updated.Role, tt.newRole)
			}
		})
	}
}

func TestUpdateDashboardUserPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test admin user
	user := createTestDashboardUser(t, db, "testadmin", "oldpassword", "admin")

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
			err := db.UpdateDashboardUserPassword(int(tt.id), tt.newPassword)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateDashboardUserPassword() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateDashboardUserPassword() unexpected error: %v", err)
			}

			// Verify the password was updated
			updated, err := db.GetDashboardUser(int(tt.id))
			if err != nil {
				t.Fatalf("GetDashboardUser() after password update failed: %v", err)
			}

			// Check if new password is correct
			err = bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte(tt.newPassword))
			if err != nil {
				t.Errorf("UpdateDashboardUserPassword() new password verification failed: %v", err)
			}
		})
	}
}

func TestDeleteDashboardUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		setup   func() uint // returns user ID to delete
		wantErr bool
	}{
		{
			name: "delete existing admin user",
			setup: func() uint {
				user := createTestDashboardUser(t, db, "todelete", "password123", "admin")
				return user.ID
			},
			wantErr: false,
		},
		{
			name: "delete non-existent admin user",
			setup: func() uint {
				return 999999
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := db.DeleteDashboardUser(int(id))

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteDashboardUser() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteDashboardUser() unexpected error: %v", err)
			}

			// Verify user is deleted
			_, err = db.GetDashboardUser(int(id))
			if err == nil {
				t.Errorf("DeleteDashboardUser() user still exists after deletion")
			}
		})
	}
}

func TestAuthenticateDashboardUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a test admin user
	createTestDashboardUser(t, db, "testadmin", "correctpassword", "admin")

	tests := []struct {
		name     string
		username string
		password string
		wantNil  bool
	}{
		{
			name:     "authenticate with correct credentials",
			username: "testadmin",
			password: "correctpassword",
			wantNil:  false,
		},
		{
			name:     "authenticate default admin",
			username: "admin",
			password: "admin",
			wantNil:  false,
		},
		{
			name:     "authenticate with wrong password",
			username: "testadmin",
			password: "wrongpassword",
			wantNil:  true,
		},
		{
			name:     "authenticate non-existent user",
			username: "nonexistent",
			password: "password123",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.AuthenticateDashboardUser(tt.username, tt.password)

			if err != nil {
				t.Fatalf("AuthenticateDashboardUser() unexpected error: %v", err)
			}

			if tt.wantNil {
				if user != nil {
					t.Errorf("AuthenticateDashboardUser() expected nil but got result")
				}
				return
			}

			if user == nil {
				t.Fatalf("AuthenticateDashboardUser() expected result but got nil")
			}

			if user.Username != tt.username {
				t.Errorf("AuthenticateDashboardUser() username = %v, want %v", user.Username, tt.username)
			}
		})
	}
}
