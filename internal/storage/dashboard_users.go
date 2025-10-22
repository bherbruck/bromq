package storage

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// CreateDashboardUser creates a new admin user
func (db *DB) CreateDashboardUser(username, password, role string) (*DashboardUser, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	if role != "admin" && role != "viewer" {
		return nil, fmt.Errorf("invalid role: must be 'admin' or 'viewer'")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &DashboardUser{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
	}

	if err := db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	return user, nil
}

// GetDashboardUser retrieves an admin user by ID
func (db *DB) GetDashboardUser(id int) (*DashboardUser, error) {
	var user DashboardUser
	if err := db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetDashboardUserByUsername retrieves an admin user by username
func (db *DB) GetDashboardUserByUsername(username string) (*DashboardUser, error) {
	var user DashboardUser
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListDashboardUsers returns all admin users
func (db *DB) ListDashboardUsers() ([]DashboardUser, error) {
	var users []DashboardUser
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateDashboardUser updates an admin user's information
func (db *DB) UpdateDashboardUser(id int, username, role string) error {
	if role != "admin" && role != "viewer" {
		return fmt.Errorf("invalid role: must be 'admin' or 'viewer'")
	}

	updates := map[string]interface{}{
		"username": username,
		"role":     role,
	}

	result := db.Model(&DashboardUser{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update admin user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}

	return nil
}

// UpdateDashboardUserPassword updates an admin user's password
func (db *DB) UpdateDashboardUserPassword(id int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	result := db.Model(&DashboardUser{}).Where("id = ?", id).Update("password_hash", string(hash))
	if result.Error != nil {
		return fmt.Errorf("failed to update password: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}

	return nil
}

// DeleteDashboardUser deletes an admin user
func (db *DB) DeleteDashboardUser(id int) error {
	result := db.Delete(&DashboardUser{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete admin user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("admin user not found")
	}

	return nil
}

// AuthenticateDashboardUser verifies admin user credentials
func (db *DB) AuthenticateDashboardUser(username, password string) (*DashboardUser, error) {
	user, err := db.GetDashboardUserByUsername(username)
	if err != nil {
		return nil, nil // User not found
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil // Invalid password
	}

	return user, nil
}
