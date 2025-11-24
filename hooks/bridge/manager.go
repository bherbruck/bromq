package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github/bherbruck/bromq/internal/storage"
)

// Manager handles MQTT bridge connections to remote brokers
type Manager struct {
	db      *storage.DB
	server  *mqttServer.Server
	bridges map[uint]*BridgeConnection // bridge ID -> connection
	ctx     context.Context            // Context for lifecycle management
	cancel  context.CancelFunc         // Cancel function for shutdown
	mu      sync.RWMutex
}

// BridgeConnection represents an active bridge connection
type BridgeConnection struct {
	bridge       *storage.Bridge
	client       BridgeClient        // Abstracted MQTT client (v3 or v5)
	inlineClient *mqttServer.Client // Inline client on local server for inbound messages
	clientID     string              // MQTT client ID for this bridge connection
	manager      *Manager
}

// NewManager creates a new bridge manager
func NewManager(db *storage.DB, server *mqttServer.Server) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		db:      db,
		server:  server,
		bridges: make(map[uint]*BridgeConnection),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// generateShortID generates a random 8-character hex ID for uniqueness
func generateShortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (extremely rare)
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xffffffff)
	}
	return hex.EncodeToString(b)
}

// Start loads all bridges from database and connects them
func (m *Manager) Start() error {
	bridges, err := m.db.ListBridges()
	if err != nil {
		return fmt.Errorf("failed to list bridges: %w", err)
	}

	slog.Info("Starting bridge connections", "count", len(bridges))

	for i := range bridges {
		bridge := &bridges[i]
		if err := m.connectBridge(bridge); err != nil {
			slog.Error("Failed to connect bridge", "name", bridge.Name, "error", err)
			// Continue with other bridges even if one fails
		}
	}

	return nil
}

// connectBridge establishes connection to a remote broker
func (m *Manager) connectBridge(bridge *storage.Bridge) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already connected
	if _, exists := m.bridges[bridge.ID]; exists {
		return fmt.Errorf("bridge %s already connected", bridge.Name)
	}

	// Ensure bridge client ID has identifying prefix for loop prevention
	// Each bridge gets a unique random ID to prevent conflicts
	clientID := bridge.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("bridge-%s", generateShortID())
	} else if len(clientID) < 7 || clientID[:7] != "bridge-" {
		clientID = fmt.Sprintf("bridge-%s", clientID)
	}

	// Create abstracted client (v3 or v5 based on bridge.MQTTVersion)
	client, err := NewBridgeClient(m.ctx, bridge, clientID)
	if err != nil {
		return fmt.Errorf("failed to create bridge client: %w", err)
	}

	// Create inline client on local server to represent bridge for inbound messages
	// This allows InjectPacket to work with proper client ID for loop prevention
	inlineClient := m.server.NewClient(nil, "bridge", clientID, true)
	m.server.Clients.Add(inlineClient)

	bc := &BridgeConnection{
		bridge:       bridge,
		client:       client,
		clientID:     clientID,
		inlineClient: inlineClient,
		manager:      m,
	}

	// Store connection
	m.bridges[bridge.ID] = bc

	// Connect to remote broker
	slog.Info("Connecting bridge", "name", bridge.Name, "remote", fmt.Sprintf("%s:%d", bridge.Host, bridge.Port), "mqtt_version", bridge.MQTTVersion)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	// Subscribe to topics for inbound direction
	for _, topic := range bridge.Topics {
		if topic.Direction == "in" || topic.Direction == "both" {
			if err := client.Subscribe(topic.Remote, topic.QoS, func(topicName string, payload []byte, qos byte, retained bool) {
				bc.handleInboundMessage(topicName, payload, qos, retained, topic)
			}); err != nil {
				slog.Error("Failed to subscribe to topic", "bridge", bridge.Name, "topic", topic.Remote, "error", err)
			} else {
				slog.Info("Bridge subscribed", "bridge", bridge.Name, "topic", topic.Remote, "qos", topic.QoS)
			}
		}
	}

	return nil
}

// handleInboundMessage processes messages received from remote broker
func (bc *BridgeConnection) handleInboundMessage(remoteTopic string, payload []byte, qos byte, retained bool, topicMapping storage.BridgeTopic) {
	// Transform topic from remote pattern to local pattern
	localTopic := TransformTopic(remoteTopic, topicMapping.Remote, topicMapping.Local)

	slog.Debug("Forwarding inbound message",
		"bridge", bc.bridge.Name,
		"remote_topic", remoteTopic,
		"local_topic", localTopic)

	// Create MQTT packet for injection
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Qos:    qos,
			Retain: retained,
		},
		TopicName: localTopic,
		Payload:   payload,
	}

	// Inject packet using bridge's inline client for proper loop prevention
	err := bc.manager.server.InjectPacket(bc.inlineClient, pk)
	if err != nil {
		slog.Error("Failed to inject inbound message",
			"bridge", bc.bridge.Name,
			"topic", localTopic,
			"error", err)
	}
}

// HandleOutboundMessage forwards a message from local broker to remote brokers
// This is called by the BridgeHook's OnPublish method
func (m *Manager) HandleOutboundMessage(topic string, payload []byte, retained bool, qos byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check each bridge to see if topic matches any outbound patterns
	for _, bc := range m.bridges {
		for _, topicMapping := range bc.bridge.Topics {
			// Only process "out" or "both" direction
			if topicMapping.Direction != "out" && topicMapping.Direction != "both" {
				continue
			}

			// Check if topic matches local pattern
			if MatchTopic(topic, topicMapping.Local) {
				// Transform to remote topic
				remoteTopic := TransformTopic(topic, topicMapping.Local, topicMapping.Remote)

				slog.Debug("Forwarding outbound message",
					"bridge", bc.bridge.Name,
					"local_topic", topic,
					"remote_topic", remoteTopic)

				// Publish to remote broker
				if err := bc.client.Publish(remoteTopic, topicMapping.QoS, retained, payload); err != nil {
					slog.Error("Failed to publish outbound message",
						"bridge", bc.bridge.Name,
						"topic", remoteTopic,
						"error", err)
				}
			}
		}
	}
}

// Stop disconnects all bridge connections
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Stopping all bridge connections", "count", len(m.bridges))

	// Cancel context to signal shutdown
	if m.cancel != nil {
		m.cancel()
	}

	for _, bc := range m.bridges {
		if err := bc.client.Disconnect(); err != nil {
			slog.Error("Error disconnecting bridge", "name", bc.bridge.Name, "error", err)
		}
		m.server.Clients.Delete(bc.clientID) // Remove inline client
		slog.Info("Bridge disconnected", "name", bc.bridge.Name)
	}

	m.bridges = make(map[uint]*BridgeConnection)
}
