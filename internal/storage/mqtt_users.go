package storage

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

// CreateMQTTUser creates a new MQTT credential
func (db *DB) CreateMQTTUser(username, password, description string, metadata datatypes.JSON) (*MQTTUser, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &MQTTUser{
		Username:     username,
		PasswordHash: string(hash),
		Description:  description,
		Metadata:     metadata,
	}

	if err := db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create MQTT user: %w", err)
	}

	return user, nil
}

// GetMQTTUser retrieves an MQTT user by ID
func (db *DB) GetMQTTUser(id int) (*MQTTUser, error) {
	var user MQTTUser
	if err := db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetMQTTUserByUsername retrieves an MQTT user by username
func (db *DB) GetMQTTUserByUsername(username string) (*MQTTUser, error) {
	var user MQTTUser
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListMQTTUsers returns all MQTT users
func (db *DB) ListMQTTUsers() ([]MQTTUser, error) {
	var users []MQTTUser
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateMQTTUser updates an MQTT user's information
func (db *DB) UpdateMQTTUser(id int, username, description string, metadata datatypes.JSON) error {
	updates := map[string]interface{}{
		"username":    username,
		"description": description,
	}

	if metadata != nil {
		updates["metadata"] = metadata
	}

	result := db.Model(&MQTTUser{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update MQTT user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("MQTT user not found")
	}

	return nil
}

// UpdateMQTTUserPassword updates an MQTT user's password
func (db *DB) UpdateMQTTUserPassword(id int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	result := db.Model(&MQTTUser{}).Where("id = ?", id).Update("password_hash", string(hash))
	if result.Error != nil {
		return fmt.Errorf("failed to update password: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("MQTT user not found")
	}

	return nil
}

// DeleteMQTTUser deletes an MQTT user and cascades to ACL rules and clients
func (db *DB) DeleteMQTTUser(id int) error {
	result := db.Delete(&MQTTUser{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete MQTT user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("MQTT user not found")
	}

	return nil
}

// AuthenticateMQTTUser verifies MQTT user credentials
func (db *DB) AuthenticateMQTTUser(username, password string) (*MQTTUser, error) {
	user, err := db.GetMQTTUserByUsername(username)
	if err != nil {
		// User not found in mqtt_users table
		return nil, fmt.Errorf("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Invalid password
		return nil, fmt.Errorf("invalid password")
	}

	return user, nil
}

// AuthenticateUser is a compatibility method for the auth hook interface
// Routes to MQTT user authentication for MQTT connections
// Returns error (not nil, nil) when authentication fails to avoid typed nil issues
func (db *DB) AuthenticateUser(username, password string) (interface{}, error) {
	user, err := db.AuthenticateMQTTUser(username, password)
	if err != nil {
		// Return nil user with error instead of (nil, nil) to avoid typed nil issues
		return nil, err
	}
	return user, nil
}

// GetMQTTUserByUsernameInterface is a wrapper that returns interface{} for hook compatibility
func (db *DB) GetMQTTUserByUsernameInterface(username string) (interface{}, error) {
	return db.GetMQTTUserByUsername(username)
}
