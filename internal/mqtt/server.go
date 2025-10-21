package mqtt

import (
	"fmt"
	"log"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// Server wraps the mochi-mqtt server
type Server struct {
	*mqtt.Server
	config *Config
}

// New creates a new MQTT server instance
func New(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	opts := &mqtt.Options{
		Capabilities: mqtt.NewDefaultServerCapabilities(),
	}

	if !cfg.RetainAvailable {
		opts.Capabilities.RetainAvailable = 0
	}

	return &Server{
		Server: mqtt.New(opts),
		config: cfg,
	}
}

// AddAuthHook adds an authentication hook to the server
func (s *Server) AddAuthHook(hook mqtt.Hook) error {
	return s.Server.AddHook(hook, nil)
}

// AddACLHook adds an ACL hook to the server
func (s *Server) AddACLHook(hook mqtt.Hook) error {
	return s.Server.AddHook(hook, nil)
}

// Start starts the MQTT server with configured listeners
func (s *Server) Start() error {
	// Add TCP listener
	if s.config.TCPAddr != "" {
		tcp := listeners.NewTCP(listeners.Config{
			ID:      "tcp",
			Address: s.config.TCPAddr,
		})
		err := s.Server.AddListener(tcp)
		if err != nil {
			return fmt.Errorf("failed to add TCP listener: %w", err)
		}
		log.Printf("MQTT TCP listener started on %s", s.config.TCPAddr)
	}

	// Add WebSocket listener
	if s.config.WSAddr != "" {
		ws := listeners.NewWebsocket(listeners.Config{
			ID:      "ws",
			Address: s.config.WSAddr,
		})
		err := s.Server.AddListener(ws)
		if err != nil {
			return fmt.Errorf("failed to add WebSocket listener: %w", err)
		}
		log.Printf("MQTT WebSocket listener started on %s", s.config.WSAddr)
	}

	// Start the server
	return s.Server.Serve()
}

// GetClients returns information about all connected clients
func (s *Server) GetClients() []ClientInfo {
	clients := s.Server.Clients.GetAll()
	info := make([]ClientInfo, 0, len(clients))

	for _, cl := range clients {
		info = append(info, ClientInfo{
			ID:               cl.ID,
			Username:         string(cl.Properties.Username),
			Remote:           cl.Net.Remote,
			Listener:         cl.Net.Listener,
			ProtocolVersion:  cl.Properties.ProtocolVersion,
			Keepalive:        cl.State.Keepalive,
			Clean:            cl.Properties.Clean,
			SubscriptionsCount: cl.State.Subscriptions.Len(),
			InflightCount:    cl.State.Inflight.Len(),
		})
	}

	return info
}

// GetClientDetails returns detailed information about a specific client
func (s *Server) GetClientDetails(clientID string) (*ClientDetails, error) {
	cl, ok := s.Server.Clients.Get(clientID)
	if !ok {
		return nil, fmt.Errorf("client not found")
	}

	// Get all subscriptions
	subs := cl.State.Subscriptions.GetAll()
	subscriptions := make([]SubscriptionInfo, 0, len(subs))
	for topic, sub := range subs {
		subscriptions = append(subscriptions, SubscriptionInfo{
			Topic: topic,
			QoS:   sub.Qos,
		})
	}

	return &ClientDetails{
		ID:               cl.ID,
		Username:         string(cl.Properties.Username),
		Remote:           cl.Net.Remote,
		Listener:         cl.Net.Listener,
		ProtocolVersion:  cl.Properties.ProtocolVersion,
		Keepalive:        cl.State.Keepalive,
		Clean:            cl.Properties.Clean,
		Subscriptions:    subscriptions,
		InflightCount:    cl.State.Inflight.Len(),
	}, nil
}

// ClientInfo holds basic information about a connected client
type ClientInfo struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	Remote             string `json:"remote"`
	Listener           string `json:"listener"`
	ProtocolVersion    byte   `json:"protocol_version"`
	Keepalive          uint16 `json:"keepalive"`
	Clean              bool   `json:"clean"`
	SubscriptionsCount int    `json:"subscriptions_count"`
	InflightCount      int    `json:"inflight_count"`
}

// ClientDetails holds detailed information about a connected client
type ClientDetails struct {
	ID              string             `json:"id"`
	Username        string             `json:"username"`
	Remote          string             `json:"remote"`
	Listener        string             `json:"listener"`
	ProtocolVersion byte               `json:"protocol_version"`
	Keepalive       uint16             `json:"keepalive"`
	Clean           bool               `json:"clean"`
	Subscriptions   []SubscriptionInfo `json:"subscriptions"`
	InflightCount   int                `json:"inflight_count"`
}

// SubscriptionInfo holds information about a client subscription
type SubscriptionInfo struct {
	Topic string `json:"topic"`
	QoS   byte   `json:"qos"`
}

// DisconnectClient forcefully disconnects a client by ID
func (s *Server) DisconnectClient(clientID string) error {
	cl, ok := s.Server.Clients.Get(clientID)
	if !ok {
		return fmt.Errorf("client not found")
	}

	cl.Stop(fmt.Errorf("disconnected by admin"))
	return nil
}

