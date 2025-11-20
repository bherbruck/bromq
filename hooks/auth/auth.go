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
	authenticator  Authenticator
	metrics        AuthMetrics
	allowAnonymous bool
}

// Authenticator interface for user authentication
type Authenticator interface {
	AuthenticateUser(username, password string) (interface{}, error)
}

// AuthMetrics interface for recording authentication metrics
type AuthMetrics interface {
	RecordAuthAttempt(username, result string)
	RecordAuthFailure(username string)
}

// NewAuthHook creates a new authentication hook
func NewAuthHook(authenticator Authenticator, allowAnonymous bool) *AuthHook {
	return &AuthHook{
		authenticator:  authenticator,
		allowAnonymous: allowAnonymous,
	}
}

// SetMetrics sets the metrics recorder (optional)
func (h *AuthHook) SetMetrics(metrics AuthMetrics) {
	h.metrics = metrics
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

	// Check anonymous connections
	if username == "" {
		if !h.allowAnonymous {
			slog.Warn("Anonymous connection rejected - anonymous access disabled", "client_id", cl.ID)
			if h.metrics != nil {
				h.metrics.RecordAuthAttempt("anonymous", "failure")
				h.metrics.RecordAuthFailure("anonymous")
			}
			return false
		}
		slog.Debug("Client connecting anonymously", "client_id", cl.ID)
		if h.metrics != nil {
			h.metrics.RecordAuthAttempt("anonymous", "success")
		}
		return true
	}

	// Authenticate user
	user, err := h.authenticator.AuthenticateUser(username, password)
	if err != nil {
		slog.Warn("Authentication failed", "username", username, "error", err)
		if h.metrics != nil {
			h.metrics.RecordAuthAttempt(username, "failure")
			h.metrics.RecordAuthFailure(username)
		}
		return false
	}

	if user == nil {
		slog.Warn("Authentication failed - user not found", "username", username)
		if h.metrics != nil {
			h.metrics.RecordAuthAttempt(username, "failure")
			h.metrics.RecordAuthFailure(username)
		}
		return false
	}

	// Username is already stored in cl.Properties.Username by mochi-mqtt
	slog.Info("Client authenticated", "client_id", cl.ID, "username", username)
	if h.metrics != nil {
		h.metrics.RecordAuthAttempt(username, "success")
	}
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
