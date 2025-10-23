package auth

import (
	"bytes"
	"log/slog"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// AuthHook implements MQTT authentication using a database
type AuthHook struct {
	mqtt.HookBase
	authenticator Authenticator
}

// Authenticator interface for user authentication
type Authenticator interface {
	AuthenticateUser(username, password string) (interface{}, error)
}


// NewAuthHook creates a new authentication hook
func NewAuthHook(authenticator Authenticator) *AuthHook {
	return &AuthHook{
		authenticator: authenticator,
	}
}

// ID returns the hook identifier
func (h *AuthHook) ID() string {
	return "database-auth"
}

// Provides indicates which hook methods this hook provides
func (h *AuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnConnect,
	}, []byte{b})
}

// OnConnectAuthenticate is called when a client attempts to connect
func (h *AuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)

	// Allow anonymous connections if no username provided
	if username == "" {
		slog.Debug("Client connecting anonymously", "client_id", cl.ID)
		return true
	}

	// Authenticate user
	user, err := h.authenticator.AuthenticateUser(username, password)
	if err != nil {
		slog.Warn("Authentication failed", "username", username, "error", err)
		return false
	}

	if user == nil {
		slog.Warn("Authentication failed - user not found", "username", username)
		return false
	}

	// Username is already stored in cl.Properties.Username by mochi-mqtt
	slog.Info("Client authenticated", "client_id", cl.ID, "username", username)
	return true
}

// OnConnect is called when a client successfully connects
func (h *AuthHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	username := string(pk.Connect.Username)
	if username == "" {
		username = "anonymous"
	}
	slog.Info("Client connected", "client_id", cl.ID, "username", username)
	return nil
}
