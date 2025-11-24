package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	pahoV3 "github.com/eclipse/paho.mqtt.golang"
	pahoV5 "github.com/eclipse/paho.golang/autopaho"
	pahoV5Client "github.com/eclipse/paho.golang/paho"
	"github/bherbruck/bromq/internal/storage"
)

// MessageHandler is called when a message is received from remote broker
type MessageHandler func(topic string, payload []byte, qos byte, retained bool)

// BridgeClient abstracts MQTT v3 and v5 clients behind a common interface
type BridgeClient interface {
	Connect() error
	Disconnect() error
	Subscribe(topic string, qos byte, handler MessageHandler) error
	Publish(topic string, qos byte, retained bool, payload []byte) error
	IsConnected() bool
}

// NewBridgeClient creates appropriate client based on MQTT version
func NewBridgeClient(ctx context.Context, bridge *storage.Bridge, clientID string) (BridgeClient, error) {
	version := bridge.MQTTVersion
	if version == "" {
		version = "5" // Default
	}

	switch version {
	case "5":
		return newV5Client(ctx, bridge, clientID)
	case "3":
		return newV3Client(bridge, clientID)
	default:
		return nil, fmt.Errorf("unsupported MQTT version: %s", version)
	}
}

// ============================================================================
// MQTT v3 Client (paho.mqtt.golang)
// ============================================================================

type v3Client struct {
	client    pahoV3.Client
	connected bool
	mu        sync.RWMutex
}

func newV3Client(bridge *storage.Bridge, clientID string) (*v3Client, error) {
	opts := pahoV3.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", bridge.Host, bridge.Port))
	opts.SetClientID(clientID)
	opts.SetUsername(bridge.Username)
	opts.SetPassword(bridge.Password)
	opts.SetCleanSession(bridge.CleanSession)
	opts.SetKeepAlive(time.Duration(bridge.KeepAlive) * time.Second)
	opts.SetConnectTimeout(time.Duration(bridge.ConnectionTimeout) * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(time.Minute)
	opts.SetResumeSubs(true)

	client := pahoV3.NewClient(opts)

	v3c := &v3Client{
		client: client,
	}

	opts.SetOnConnectHandler(func(c pahoV3.Client) {
		v3c.mu.Lock()
		v3c.connected = true
		v3c.mu.Unlock()
		slog.Info("MQTT v3 bridge connected", "client_id", clientID)
	})

	opts.SetConnectionLostHandler(func(c pahoV3.Client, err error) {
		v3c.mu.Lock()
		v3c.connected = false
		v3c.mu.Unlock()
		slog.Warn("MQTT v3 bridge connection lost", "client_id", clientID, "error", err)
	})

	return v3c, nil
}

func (c *v3Client) Connect() error {
	token := c.client.Connect()
	if !token.WaitTimeout(30 * time.Second) {
		return fmt.Errorf("connection timeout")
	}
	return token.Error()
}

func (c *v3Client) Disconnect() error {
	c.client.Disconnect(250)
	return nil
}

func (c *v3Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	token := c.client.Subscribe(topic, qos, func(client pahoV3.Client, msg pahoV3.Message) {
		handler(msg.Topic(), msg.Payload(), msg.Qos(), msg.Retained())
	})
	token.Wait()
	return token.Error()
}

func (c *v3Client) Publish(topic string, qos byte, retained bool, payload []byte) error {
	token := c.client.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

func (c *v3Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// ============================================================================
// MQTT v5 Client (paho.golang/autopaho)
// ============================================================================

type v5Client struct {
	cm              *pahoV5.ConnectionManager
	ctx             context.Context
	clientID        string
	subscriptions   map[string]MessageHandler // topic -> handler
	mu              sync.RWMutex
}

func newV5Client(ctx context.Context, bridge *storage.Bridge, clientID string) (*v5Client, error) {
	serverURL, err := url.Parse(fmt.Sprintf("mqtt://%s:%d", bridge.Host, bridge.Port))
	if err != nil {
		return nil, fmt.Errorf("invalid broker URL: %w", err)
	}

	v5c := &v5Client{
		ctx:           ctx,
		clientID:      clientID,
		subscriptions: make(map[string]MessageHandler),
	}

	// Validate KeepAlive fits in uint16 (MQTT v5 spec)
	keepAlive := bridge.KeepAlive
	if keepAlive > 65535 {
		keepAlive = 65535
	}

	cfg := pahoV5.ClientConfig{
		ServerUrls: []*url.URL{serverURL},
		KeepAlive:  uint16(keepAlive), // #nosec G115 - validated above
		ConnectTimeout: time.Duration(bridge.ConnectionTimeout) * time.Second,
		CleanStartOnInitialConnection: bridge.CleanSession,
		ConnectUsername: bridge.Username,
		ConnectPassword: []byte(bridge.Password),

		ConnectPacketBuilder: func(p *pahoV5Client.Connect, u *url.URL) (*pahoV5Client.Connect, error) {
			p.ClientID = clientID
			p.CleanStart = bridge.CleanSession
			p.KeepAlive = uint16(keepAlive) // #nosec G115 - validated above
			return p, nil
		},

		OnConnectionUp: func(cm *pahoV5.ConnectionManager, connack *pahoV5Client.Connack) {
			slog.Info("MQTT v5 bridge connected", "client_id", clientID, "session_present", connack.SessionPresent)
			// autopaho with SetResumeSubs handles resubscription automatically
		},

		OnConnectError: func(err error) {
			slog.Error("MQTT v5 bridge connection error", "client_id", clientID, "error", err)
		},

		ClientConfig: pahoV5Client.ClientConfig{
			OnPublishReceived: []func(pahoV5Client.PublishReceived) (bool, error){
				func(pr pahoV5Client.PublishReceived) (bool, error) {
					// Route message to appropriate handler based on topic
					v5c.mu.RLock()
					defer v5c.mu.RUnlock()

					for subTopic, handler := range v5c.subscriptions {
						// TODO: Proper topic matching with wildcards
						// For now, use exact match or prefix check
						if matchTopic(subTopic, pr.Packet.Topic) {
							handler(pr.Packet.Topic, pr.Packet.Payload, pr.Packet.QoS, pr.Packet.Retain)
							return true, nil // Acknowledge
						}
					}
					return true, nil
				},
			},
		},
	}

	cm, err := pahoV5.NewConnection(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create v5 connection: %w", err)
	}

	v5c.cm = cm
	return v5c, nil
}

func (c *v5Client) Connect() error {
	// Connection is established in NewConnection, wait for it
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	if err := c.cm.AwaitConnection(ctx); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	return nil
}

func (c *v5Client) Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	return c.cm.Disconnect(ctx)
}

func (c *v5Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	c.mu.Lock()
	c.subscriptions[topic] = handler
	c.mu.Unlock()

	// Subscribe with NoLocal to prevent receiving own messages (loop prevention!)
	_, err := c.cm.Subscribe(context.Background(), &pahoV5Client.Subscribe{
		Subscriptions: []pahoV5Client.SubscribeOptions{
			{
				Topic:   topic,
				QoS:     qos,
				NoLocal: true, // KEY FEATURE: Prevents message loops!
			},
		},
	})

	if err != nil {
		return fmt.Errorf("subscribe failed: %w", err)
	}

	slog.Debug("MQTT v5 subscribed with NoLocal", "topic", topic, "qos", qos)
	return nil
}

func (c *v5Client) Publish(topic string, qos byte, retained bool, payload []byte) error {
	_, err := c.cm.Publish(context.Background(), &pahoV5Client.Publish{
		Topic:   topic,
		QoS:     qos,
		Retain:  retained,
		Payload: payload,
	})
	return err
}

func (c *v5Client) IsConnected() bool {
	// autopaho manages connection state internally
	// We'll use AwaitConnection with a short timeout to check
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Millisecond)
	defer cancel()
	return c.cm.AwaitConnection(ctx) == nil
}

// matchTopic checks if a topic matches a subscription pattern
// This is a simplified implementation - uses the existing MatchTopic from topic.go
func matchTopic(pattern, topic string) bool {
	return MatchTopic(topic, pattern) // MatchTopic expects (topic, pattern)
}
