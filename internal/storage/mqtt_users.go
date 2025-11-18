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

	// Add to cache immediately
	db.cache.SetMQTTUser(username, user)

	return user, nil
}

// GetMQTTUser retrieves an MQTT user by ID
func (db *DB) GetMQTTUser(id uint) (*MQTTUser, error) {
	var user MQTTUser
	if err := db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetMQTTUserByUsername retrieves an MQTT user by username
// Uses in-memory cache to avoid database queries on hot path (MQTT pub/sub)
func (db *DB) GetMQTTUserByUsername(username string) (*MQTTUser, error) {
	// Check cache first
	if cachedUser, found := db.cache.GetMQTTUser(username); found {
		return cachedUser, nil
	}

	// Cache miss - query database
	var user MQTTUser
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}

	// Store in cache for future requests
	db.cache.SetMQTTUser(username, &user)

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

// ListMQTTUsersPaginated returns paginated MQTT users with search and sorting
func (db *DB) ListMQTTUsersPaginated(page, pageSize int, search, sortBy, sortOrder string) ([]MQTTUser, int64, error) {
	var users []MQTTUser
	var total int64

	query := db.Model(&MQTTUser{})

	// Apply search filter
	if search != "" {
		query = query.Where("username LIKE ? OR description LIKE ?",
			"%"+search+"%", "%"+search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count MQTT users: %w", err)
	}

	// Apply sorting
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" || (sortOrder != "asc" && sortOrder != "desc") {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize)

	// Execute query
	if err := query.Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list MQTT users: %w", err)
	}

	return users, total, nil
}

// UpdateMQTTUser updates an MQTT user's information
func (db *DB) UpdateMQTTUser(id uint, username, description string, metadata datatypes.JSON) error {
	// Get old username to invalidate cache
	oldUser, err := db.GetMQTTUser(id)
	if err != nil {
		return fmt.Errorf("MQTT user not found")
	}

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

	// Invalidate cache for old username
	db.cache.DeleteMQTTUser(oldUser.Username)
	// If username changed, invalidate new username too (for safety)
	if username != oldUser.Username {
		db.cache.DeleteMQTTUser(username)
	}

	return nil
}

// UpdateMQTTUserPassword updates an MQTT user's password
func (db *DB) UpdateMQTTUserPassword(id uint, password string) error {
	// Get username to invalidate cache
	user, err := db.GetMQTTUser(id)
	if err != nil {
		return fmt.Errorf("MQTT user not found")
	}

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

	// Invalidate cache (password changed)
	db.cache.DeleteMQTTUser(user.Username)

	return nil
}

// DeleteMQTTUser deletes an MQTT user and cascades to ACL rules and clients
func (db *DB) DeleteMQTTUser(id uint) error {
	// Get username to invalidate cache
	user, err := db.GetMQTTUser(id)
	if err != nil {
		return fmt.Errorf("MQTT user not found")
	}

	result := db.Delete(&MQTTUser{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete MQTT user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("MQTT user not found")
	}

	// Invalidate cache and ACL rules for this user
	db.cache.DeleteMQTTUser(user.Username)
	db.cache.DeleteACLRules(user.ID)

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

// MarkAsProvisioned marks an MQTT user as provisioned from config file
func (db *DB) MarkAsProvisioned(id uint, provisioned bool) error {
	// Get username to invalidate cache
	user, err := db.GetMQTTUser(id)
	if err != nil {
		return fmt.Errorf("MQTT user not found")
	}

	result := db.Model(&MQTTUser{}).Where("id = ?", id).Update("provisioned_from_config", provisioned)
	if result.Error != nil {
		return fmt.Errorf("failed to mark user as provisioned: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("MQTT user not found")
	}

	// Invalidate cache so next read gets updated value
	db.cache.DeleteMQTTUser(user.Username)

	return nil
}

// ListProvisionedMQTTUsers returns all MQTT users that were provisioned from config
func (db *DB) ListProvisionedMQTTUsers() ([]MQTTUser, error) {
	var users []MQTTUser
	if err := db.Where("provisioned_from_config = ?", true).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
