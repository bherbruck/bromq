package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github/bherbruck/bromq/internal/storage"
)

// Manager handles MQTT bridge connections to remote brokers
type Manager struct {
	db      *storage.DB
	server  *mqttServer.Server
	bridges map[uint]*BridgeConnection // bridge ID -> connection
	mu      sync.RWMutex
}

// BridgeConnection represents an active bridge connection
type BridgeConnection struct {
	bridge       *storage.Bridge
	client       mqtt.Client               // Paho client connected to remote broker
	inlineClient *mqttServer.Client        // Inline client on local server for inbound messages
	clientID     string                    // MQTT client ID for this bridge connection
	manager      *Manager
	reconnecting bool
	mu           sync.Mutex
}

// NewManager creates a new bridge manager
func NewManager(db *storage.DB, server *mqttServer.Server) *Manager {
	return &Manager{
		db:      db,
		server:  server,
		bridges: make(map[uint]*BridgeConnection),
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

	// Create paho MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", bridge.Host, bridge.Port))

	// Ensure bridge client ID has identifying prefix for loop prevention
	// Each bridge gets a unique random ID to prevent conflicts
	clientID := bridge.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("bridge-%s", generateShortID())
	} else if len(clientID) < 7 || clientID[:7] != "bridge-" {
		clientID = fmt.Sprintf("bridge-%s", clientID)
	}
	opts.SetClientID(clientID)
	opts.SetUsername(bridge.Username)
	opts.SetPassword(bridge.Password)
	opts.SetCleanSession(bridge.CleanSession)
	opts.SetKeepAlive(time.Duration(bridge.KeepAlive) * time.Second)
	opts.SetConnectTimeout(time.Duration(bridge.ConnectionTimeout) * time.Second)
	opts.SetAutoReconnect(true) // Paho handles reconnection and keep-alive
	opts.SetMaxReconnectInterval(time.Minute)
	opts.SetResumeSubs(true) // Resume subscriptions after reconnect

	bc := &BridgeConnection{
		bridge:   bridge,
		clientID: clientID, // Store for loop prevention
		manager:  m,
	}

	// Set connection callbacks
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		bc.onConnect(client)
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		bc.onConnectionLost(err)
	})

	// Create Paho client for connecting to remote broker
	client := mqtt.NewClient(opts)
	bc.client = client

	// Create inline client on local server to represent bridge for inbound messages
	// This allows InjectPacket to work with proper client ID for loop prevention
	inlineClient := m.server.NewClient(nil, "bridge", clientID, true)
	m.server.Clients.Add(inlineClient)
	bc.inlineClient = inlineClient

	// Store connection
	m.bridges[bridge.ID] = bc

	// Connect
	slog.Info("Connecting bridge", "name", bridge.Name, "remote", fmt.Sprintf("%s:%d", bridge.Host, bridge.Port))
	token := client.Connect()
	if !token.WaitTimeout(time.Duration(bridge.ConnectionTimeout) * time.Second) {
		return fmt.Errorf("connection timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	return nil
}

// onConnect is called when bridge successfully connects to remote broker
func (bc *BridgeConnection) onConnect(client mqtt.Client) {
	slog.Info("Bridge connected", "name", bc.bridge.Name)

	// Subscribe to remote topics for "in" direction
	for _, topic := range bc.bridge.Topics {
		if topic.Direction == "in" || topic.Direction == "both" {
			// Subscribe to remote pattern
			token := client.Subscribe(topic.Remote, topic.QoS, func(c mqtt.Client, msg mqtt.Message) {
				bc.handleInboundMessage(msg, topic)
			})
			token.Wait()
			if err := token.Error(); err != nil {
				slog.Error("Failed to subscribe to remote topic",
					"bridge", bc.bridge.Name,
					"topic", topic.Remote,
					"error", err)
			} else {
				slog.Debug("Subscribed to remote topic",
					"bridge", bc.bridge.Name,
					"topic", topic.Remote)
			}
		}
	}

	// Reset reconnection flag
	bc.mu.Lock()
	bc.reconnecting = false
	bc.mu.Unlock()
}

// onConnectionLost is called when bridge loses connection to remote broker
func (bc *BridgeConnection) onConnectionLost(err error) {
	slog.Warn("Bridge connection lost", "name", bc.bridge.Name, "error", err)

	// Start reconnection with exponential backoff
	go bc.reconnect()
}

// reconnect attempts to reconnect with exponential backoff
func (bc *BridgeConnection) reconnect() {
	bc.mu.Lock()
	if bc.reconnecting {
		bc.mu.Unlock()
		return // Already reconnecting
	}
	bc.reconnecting = true
	bc.mu.Unlock()

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		slog.Info("Attempting to reconnect bridge", "name", bc.bridge.Name, "backoff", backoff)

		token := bc.client.Connect()
		token.WaitTimeout(time.Duration(bc.bridge.ConnectionTimeout) * time.Second)
		if err := token.Error(); err == nil {
			// Successfully reconnected
			slog.Info("Bridge reconnected successfully", "name", bc.bridge.Name)
			return
		}

		slog.Debug("Reconnection failed, retrying", "name", bc.bridge.Name, "backoff", backoff)

		// Wait before next attempt
		time.Sleep(backoff)

		// Increase backoff exponentially
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// handleInboundMessage processes messages received from remote broker
func (bc *BridgeConnection) handleInboundMessage(msg mqtt.Message, topicMapping storage.BridgeTopic) {
	// Transform topic from remote pattern to local pattern
	localTopic := TransformTopic(msg.Topic(), topicMapping.Remote, topicMapping.Local)

	slog.Debug("Forwarding inbound message",
		"bridge", bc.bridge.Name,
		"remote_topic", msg.Topic(),
		"local_topic", localTopic)

	// Create MQTT packet for injection
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Qos:    msg.Qos(),
			Retain: msg.Retained(),
		},
		TopicName: localTopic,
		Payload:   msg.Payload(),
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
				token := bc.client.Publish(remoteTopic, topicMapping.QoS, retained, payload)
				go func(bridgeName string) {
					token.Wait()
					if err := token.Error(); err != nil {
						slog.Error("Failed to publish outbound message",
							"bridge", bridgeName,
							"topic", remoteTopic,
							"error", err)
					}
				}(bc.bridge.Name)
			}
		}
	}
}

// Stop disconnects all bridge connections
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Stopping all bridge connections", "count", len(m.bridges))

	for _, bc := range m.bridges {
		bc.client.Disconnect(250) // 250ms grace period
		m.server.Clients.Delete(bc.clientID) // Remove inline client
		slog.Info("Bridge disconnected", "name", bc.bridge.Name)
	}

	m.bridges = make(map[uint]*BridgeConnection)
}
