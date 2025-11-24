package bridge

import (
	"bytes"
	"strings"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// BridgeHook intercepts local MQTT publishes and forwards matching topics to remote brokers
type BridgeHook struct {
	mqtt.HookBase
	manager *Manager
}

// NewBridgeHook creates a new bridge hook
func NewBridgeHook(manager *Manager) *BridgeHook {
	return &BridgeHook{
		manager: manager,
	}
}

// ID returns the hook identifier
func (h *BridgeHook) ID() string {
	return "mqtt-bridge"
}

// Provides indicates which hook methods this hook provides
func (h *BridgeHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnPublish,
	}, []byte{b})
}

// OnPublish is called when a message is published locally
// It checks if the topic matches any bridge patterns and forwards to remote brokers
func (h *BridgeHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// Loop prevention: Skip forwarding if message originated from a bridge connection
	// Bridge client IDs are prefixed with "bridge-"
	if strings.HasPrefix(cl.ID, "bridge-") {
		// Message came from a remote broker via bridge, don't forward back
		return pk, nil
	}

	// Forward message to bridge manager for outbound routing
	h.manager.HandleOutboundMessage(
		pk.TopicName,
		pk.Payload,
		pk.FixedHeader.Retain,
		pk.FixedHeader.Qos,
	)

	// Return unchanged packet to continue normal local delivery
	return pk, nil
}
