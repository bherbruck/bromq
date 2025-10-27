package storage

import (
	"fmt"
	"time"

	"gorm.io/datatypes"
)

// UpsertMQTTClient creates or updates an MQTT client record
// Used when a client connects to track first/last seen times
func (db *DB) UpsertMQTTClient(clientID string, mqttUserID uint, metadata datatypes.JSON) (*MQTTClient, error) {
	var client MQTTClient
	now := time.Now()

	// Try to find existing client
	err := db.Where("client_id = ?", clientID).First(&client).Error
	if err != nil {
		// Client doesn't exist, create new
		client = MQTTClient{
			ClientID:   clientID,
			MQTTUserID: mqttUserID,
			Metadata:   metadata,
			FirstSeen:  now,
			LastSeen:   now,
			IsActive:   true,
		}

		if err := db.Create(&client).Error; err != nil {
			return nil, fmt.Errorf("failed to create MQTT client: %w", err)
		}
	} else {
		// Client exists, update last seen and active status
		updates := map[string]interface{}{
			"last_seen": now,
			"is_active": true,
		}

		// Update MQTTUserID if it changed (client reconnected with different credentials)
		if client.MQTTUserID != mqttUserID {
			updates["mqtt_user_id"] = mqttUserID
		}

		// Update metadata if provided
		if metadata != nil {
			updates["metadata"] = metadata
		}

		if err := db.Model(&client).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update MQTT client: %w", err)
		}
	}

	return &client, nil
}

// MarkMQTTClientInactive marks a client as disconnected
func (db *DB) MarkMQTTClientInactive(clientID string) error {
	result := db.Model(&MQTTClient{}).
		Where("client_id = ?", clientID).
		Updates(map[string]interface{}{
			"is_active": false,
			"last_seen": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark client inactive: %w", result.Error)
	}

	return nil
}

// GetMQTTClient retrieves a client by ID
func (db *DB) GetMQTTClient(id int) (*MQTTClient, error) {
	var client MQTTClient
	if err := db.Preload("MQTTUser").First(&client, id).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

// GetMQTTClientByClientID retrieves a client by client ID
func (db *DB) GetMQTTClientByClientID(clientID string) (*MQTTClient, error) {
	var client MQTTClient
	if err := db.Preload("MQTTUser").Where("client_id = ?", clientID).First(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

// ListMQTTClients returns all MQTT clients with optional filters
func (db *DB) ListMQTTClients(activeOnly bool) ([]MQTTClient, error) {
	var clients []MQTTClient
	query := db.Preload("MQTTUser")

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Order("last_seen DESC").Find(&clients).Error; err != nil {
		return nil, err
	}

	return clients, nil
}

// ListMQTTClientsPaginated returns paginated MQTT clients with optional search and sorting
func (db *DB) ListMQTTClientsPaginated(page, pageSize int, search, sortBy, sortOrder string, activeOnly bool) ([]MQTTClient, int64, error) {
	var clients []MQTTClient
	var total int64

	query := db.Model(&MQTTClient{}).Preload("MQTTUser")

	// Apply active filter
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	// Apply search filter (search in client_id)
	if search != "" {
		query = query.Where("client_id LIKE ?", "%"+search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count MQTT clients: %w", err)
	}

	// Apply sorting
	if sortBy == "" {
		sortBy = "last_seen"
	}
	if sortOrder == "" || (sortOrder != "asc" && sortOrder != "desc") {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize)

	if err := query.Find(&clients).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list MQTT clients: %w", err)
	}

	return clients, total, nil
}

// ListMQTTClientsByUser returns all clients for a specific MQTT user
func (db *DB) ListMQTTClientsByUser(mqttUserID uint, activeOnly bool) ([]MQTTClient, error) {
	var clients []MQTTClient
	query := db.Where("mqtt_user_id = ?", mqttUserID)

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Order("last_seen DESC").Find(&clients).Error; err != nil {
		return nil, err
	}

	return clients, nil
}

// UpdateMQTTClientMetadata updates a client's metadata
func (db *DB) UpdateMQTTClientMetadata(clientID string, metadata datatypes.JSON) error {
	result := db.Model(&MQTTClient{}).
		Where("client_id = ?", clientID).
		Update("metadata", metadata)

	if result.Error != nil {
		return fmt.Errorf("failed to update client metadata: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// DeleteMQTTClient deletes a client record
func (db *DB) DeleteMQTTClient(id int) error {
	result := db.Delete(&MQTTClient{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete MQTT client: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// GetClientCount returns the number of clients (active or total)
func (db *DB) GetClientCount(activeOnly bool) (int64, error) {
	var count int64
	query := db.Model(&MQTTClient{})

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// UpsertMQTTClientInterface is a wrapper that accepts interface{} metadata for hook compatibility
func (db *DB) UpsertMQTTClientInterface(clientID string, mqttUserID uint, metadata interface{}) (interface{}, error) {
	var jsonMetadata datatypes.JSON
	if metadata != nil {
		if jsonMeta, ok := metadata.(datatypes.JSON); ok {
			jsonMetadata = jsonMeta
		}
	}
	return db.UpsertMQTTClient(clientID, mqttUserID, jsonMetadata)
}
