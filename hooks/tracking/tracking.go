package tracking

import (
	"bytes"
	"log/slog"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// ClientTracker interface for tracking MQTT client connections
type ClientTracker interface {
	UpsertMQTTClientInterface(clientID string, mqttUserID uint, metadata interface{}) (interface{}, error)
	MarkMQTTClientInactive(clientID string) error
	GetMQTTUserByUsernameInterface(username string) (interface{}, error)
}

// TrackingHook implements MQTT client tracking using a database
type TrackingHook struct {
	mqtt.HookBase
	tracker ClientTracker
}

// New AuthHook creates a new authentication hook
func NewTrackingHook(tracker ClientTracker) *TrackingHook {
	return &TrackingHook{
		tracker: tracker,
	}
}

// ID returns the hook identifier
func (h *TrackingHook) ID() string {
	return "client-tracking"
}

// Provides indicates which hook methods this hook provides
func (h *TrackingHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
	}, []byte{b})
}

// OnConnect is called when a client successfully connects
// This creates or updates the client record in the database
func (h *TrackingHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	username := string(pk.Connect.Username)
	if username == "" {
		// Anonymous connection - don't track
		return nil
	}

	// Get the MQTT user ID for this username
	userInterface, err := h.tracker.GetMQTTUserByUsernameInterface(username)
	if err != nil {
		slog.Warn("Failed to get MQTT user for tracking", "error", err)
		return nil // Don't fail the connection
	}

	if userInterface == nil {
		// User not found (shouldn't happen after successful auth)
		return nil
	}

	// Extract user ID using type assertion
	// We expect a struct with an ID field
	type HasID interface {
		GetID() uint
	}

	var mqttUserID uint
	if hasID, ok := userInterface.(HasID); ok {
		mqttUserID = hasID.GetID()
	} else {
		// Try direct field access via reflection
		// For now, just log and skip
		slog.Warn("Unable to extract user ID", "type", userInterface)
		return nil
	}

	// Create or update client record
	_, err = h.tracker.UpsertMQTTClientInterface(cl.ID, mqttUserID, nil)
	if err != nil {
		slog.Warn("Failed to track client connection", "error", err)
		return nil // Don't fail the connection
	}

	slog.Debug("Client connection tracked", "client_id", cl.ID, "username", username)
	return nil
}

// OnDisconnect is called when a client disconnects
// This marks the client as inactive
func (h *TrackingHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	if err := h.tracker.MarkMQTTClientInactive(cl.ID); err != nil {
		slog.Warn("Failed to mark client as inactive", "client_id", cl.ID, "error", err)
	} else {
		slog.Debug("Client marked as disconnected", "client_id", cl.ID)
	}
}
