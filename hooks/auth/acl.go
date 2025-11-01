package auth

import (
	"bytes"
	"log/slog"

	mqtt "github.com/mochi-mqtt/server/v2"
)

// ACLHook implements MQTT ACL (Access Control List) using a database
type ACLHook struct {
	mqtt.HookBase
	checker ACLChecker
	metrics ACLMetrics
}

// ACLChecker interface for checking ACL permissions
// Supports dynamic placeholders: ${username} and ${clientid}
type ACLChecker interface {
	CheckACL(username, clientID, topic, action string) (bool, error)
}

// ACLMetrics interface for recording ACL metrics
type ACLMetrics interface {
	RecordACLCheck(username, action, result string)
	RecordACLDenied(username, action, topic string)
}

// NewACLHook creates a new ACL hook
func NewACLHook(checker ACLChecker) *ACLHook {
	return &ACLHook{
		checker: checker,
	}
}

// SetMetrics sets the metrics recorder (optional)
func (h *ACLHook) SetMetrics(metrics ACLMetrics) {
	h.metrics = metrics
}

// ID returns the hook identifier
func (h *ACLHook) ID() string {
	return "database-acl"
}

// Provides indicates which hook methods this hook provides
func (h *ACLHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnACLCheck,
	}, []byte{b})
}

// OnACLCheck is called when a client attempts to publish or subscribe
func (h *ACLHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	// Get username from client properties
	username := string(cl.Properties.Username)
	if username == "" {
		username = "anonymous"
	}

	// Get client ID
	clientID := cl.ID

	// Determine action (publish or subscribe)
	action := "sub"
	if write {
		action = "pub"
	}

	// Check ACL with placeholder support
	allowed, err := h.checker.CheckACL(username, clientID, topic, action)
	if err != nil {
		slog.Error("ACL check error", "username", username, "clientid", clientID, "topic", topic, "action", action, "error", err)
		if h.metrics != nil {
			h.metrics.RecordACLCheck(username, action, "error")
		}
		return false
	}

	// Record metrics
	if h.metrics != nil {
		if allowed {
			h.metrics.RecordACLCheck(username, action, "allowed")
		} else {
			h.metrics.RecordACLCheck(username, action, "denied")
			h.metrics.RecordACLDenied(username, action, topic)
			slog.Warn("ACL denied", "username", username, "clientid", clientID, "topic", topic, "action", action)
		}
	}

	return allowed
}
