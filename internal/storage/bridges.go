package storage

import (
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreateBridge creates a new MQTT bridge with its topic mappings
func (db *DB) CreateBridge(
	name, host string,
	port int,
	username, password string,
	clientID string,
	cleanSession bool,
	keepAlive, connectionTimeout int,
	metadata datatypes.JSON,
	topics []BridgeTopic,
) (*Bridge, error) {
	if name == "" || host == "" {
		return nil, fmt.Errorf("name and host are required")
	}

	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", port)
	}

	// Validate topics
	for _, topic := range topics {
		if topic.Local == "" || topic.Remote == "" {
			return nil, fmt.Errorf("local and remote are required for all topics")
		}
		if topic.Direction != "in" && topic.Direction != "out" && topic.Direction != "both" {
			return nil, fmt.Errorf("invalid direction: %s (must be 'in', 'out', or 'both')", topic.Direction)
		}
	}

	bridge := &Bridge{
		Name:              name,
		Host:              host,
		Port:              port,
		Username:          username,
		Password:          password, // Stored in plain text for outbound connections
		ClientID:          clientID,
		CleanSession:      cleanSession,
		KeepAlive:         keepAlive,
		ConnectionTimeout: connectionTimeout,
		Metadata:          metadata,
		Topics:            topics,
	}

	if err := db.Create(bridge).Error; err != nil {
		return nil, fmt.Errorf("failed to create bridge: %w", err)
	}

	return bridge, nil
}

// GetBridge retrieves a bridge by ID with its topics preloaded
func (db *DB) GetBridge(id uint) (*Bridge, error) {
	var bridge Bridge
	if err := db.Preload("Topics").First(&bridge, id).Error; err != nil {
		return nil, err
	}
	return &bridge, nil
}

// GetBridgeByName retrieves a bridge by name with its topics preloaded
func (db *DB) GetBridgeByName(name string) (*Bridge, error) {
	var bridge Bridge
	if err := db.Preload("Topics").Where("name = ?", name).First(&bridge).Error; err != nil {
		return nil, err
	}
	return &bridge, nil
}

// ListBridges returns all bridges with their topics preloaded
func (db *DB) ListBridges() ([]Bridge, error) {
	var bridges []Bridge
	if err := db.Preload("Topics").Find(&bridges).Error; err != nil {
		return nil, err
	}
	return bridges, nil
}

// ListBridgesPaginated returns a paginated list of bridges with optional search
func (db *DB) ListBridgesPaginated(page, pageSize int, search, sortBy, sortOrder string) ([]Bridge, int64, error) {
	var bridges []Bridge
	var total int64

	query := db.Model(&Bridge{})

	// Apply search filter (search by name or host)
	if search != "" {
		query = query.Where("name LIKE ? OR host LIKE ?",
			"%"+search+"%", "%"+search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count bridges: %w", err)
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

	// Execute query with preloaded topics
	if err := query.Preload("Topics").Find(&bridges).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list bridges: %w", err)
	}

	return bridges, total, nil
}

// UpdateBridge updates a bridge's configuration
// Note: This function DOES NOT update topics - use UpdateBridgeTopics for that
// Provisioned bridges cannot be updated via API (use config file instead)
func (db *DB) UpdateBridge(
	id uint,
	name, host string,
	port int,
	username, password string,
	clientID string,
	cleanSession bool,
	keepAlive, connectionTimeout int,
	metadata datatypes.JSON,
) (*Bridge, error) {
	bridge, err := db.GetBridge(id)
	if err != nil {
		return nil, fmt.Errorf("bridge not found: %w", err)
	}

	// Check if this bridge is provisioned from config - only block API updates
	// Provisioning process uses updateBridgeInternal which bypasses this check
	if bridge.ProvisionedFromConfig {
		return nil, fmt.Errorf("cannot modify bridge '%s': it is provisioned from config file", bridge.Name)
	}

	return db.updateBridgeInternal(id, name, host, port, username,
		password, clientID, cleanSession, keepAlive, connectionTimeout, metadata)
}

// updateBridgeInternal performs the actual update without provisioning checks
// Used internally by both UpdateBridge (API) and provisioning
func (db *DB) updateBridgeInternal(
	id uint,
	name, host string,
	port int,
	username, password string,
	clientID string,
	cleanSession bool,
	keepAlive, connectionTimeout int,
	metadata datatypes.JSON,
) (*Bridge, error) {
	if name == "" || host == "" {
		return nil, fmt.Errorf("name and host are required")
	}

	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", port)
	}

	updates := map[string]interface{}{
		"name":               name,
		"host":               host,
		"port":               port,
		"username":           username,
		"password":           password,
		"client_id":          clientID,
		"clean_session":      cleanSession,
		"keep_alive":         keepAlive,
		"connection_timeout": connectionTimeout,
		"metadata":           metadata,
	}

	if err := db.Model(&Bridge{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update bridge: %w", err)
	}

	return db.GetBridge(id)
}

// UpdateBridgeTopics replaces all topics for a bridge
func (db *DB) UpdateBridgeTopics(id uint, topics []BridgeTopic) error {
	bridge, err := db.GetBridge(id)
	if err != nil {
		return fmt.Errorf("bridge not found: %w", err)
	}

	// Check if this bridge is provisioned from config
	if bridge.ProvisionedFromConfig {
		return fmt.Errorf("cannot modify bridge '%s': it is provisioned from config file", bridge.Name)
	}

	// Validate topics
	for _, topic := range topics {
		if topic.Local == "" || topic.Remote == "" {
			return fmt.Errorf("local and remote are required for all topics")
		}
		if topic.Direction != "in" && topic.Direction != "out" && topic.Direction != "both" {
			return fmt.Errorf("invalid direction: %s (must be 'in', 'out', or 'both')", topic.Direction)
		}
	}

	// Delete existing topics and create new ones in a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Delete old topics
		if err := tx.Where("bridge_id = ?", id).Delete(&BridgeTopic{}).Error; err != nil {
			return fmt.Errorf("failed to delete old topics: %w", err)
		}

		// Create new topics
		for i := range topics {
			topics[i].BridgeID = id
		}
		if len(topics) > 0 {
			if err := tx.Create(&topics).Error; err != nil {
				return fmt.Errorf("failed to create new topics: %w", err)
			}
		}

		return nil
	})
}

// DeleteBridge deletes a bridge and its topics (cascade)
func (db *DB) DeleteBridge(id uint) error {
	bridge, err := db.GetBridge(id)
	if err != nil {
		return fmt.Errorf("bridge not found: %w", err)
	}

	// Check if this bridge is provisioned from config
	if bridge.ProvisionedFromConfig {
		return fmt.Errorf("cannot delete bridge '%s': it is provisioned from config file", bridge.Name)
	}

	if err := db.Delete(&Bridge{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete bridge: %w", err)
	}

	return nil
}

// GetBridgeTopics returns all topics for a specific bridge
func (db *DB) GetBridgeTopics(bridgeID uint) ([]BridgeTopic, error) {
	var topics []BridgeTopic
	if err := db.Where("bridge_id = ?", bridgeID).Find(&topics).Error; err != nil {
		return nil, fmt.Errorf("failed to get bridge topics: %w", err)
	}
	return topics, nil
}

// DeleteBridgesProvisionedFromConfig deletes all bridges that were provisioned from config
// This is used during provisioning to clean up old config items
func (db *DB) DeleteBridgesProvisionedFromConfig() error {
	// Delete topics first (they have foreign key to bridges)
	if err := db.Where("bridge_id IN (SELECT id FROM bridges WHERE provisioned_from_config = ?)", true).
		Delete(&BridgeTopic{}).Error; err != nil {
		return fmt.Errorf("failed to delete provisioned bridge topics: %w", err)
	}

	// Delete bridges
	if err := db.Where("provisioned_from_config = ?", true).Delete(&Bridge{}).Error; err != nil {
		return fmt.Errorf("failed to delete provisioned bridges: %w", err)
	}

	return nil
}

// MarkBridgeAsProvisioned marks a bridge as provisioned from config
func (db *DB) MarkBridgeAsProvisioned(id uint, provisioned bool) error {
	return db.Model(&Bridge{}).Where("id = ?", id).Update("provisioned_from_config", provisioned).Error
}

// ListProvisionedBridges returns all bridges that were provisioned from config
func (db *DB) ListProvisionedBridges() ([]Bridge, error) {
	var bridges []Bridge
	if err := db.Where("provisioned_from_config = ?", true).Preload("Topics").Find(&bridges).Error; err != nil {
		return nil, fmt.Errorf("failed to list provisioned bridges: %w", err)
	}
	return bridges, nil
}
