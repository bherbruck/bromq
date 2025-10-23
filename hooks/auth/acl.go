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
}

// ACLChecker interface for checking ACL permissions
type ACLChecker interface {
	CheckACL(username, topic, action string) (bool, error)
}

// NewACLHook creates a new ACL hook
func NewACLHook(checker ACLChecker) *ACLHook {
	return &ACLHook{
		checker: checker,
	}
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

	// Determine action (publish or subscribe)
	action := "sub"
	if write {
		action = "pub"
	}

	// Check ACL
	allowed, err := h.checker.CheckACL(username, topic, action)
	if err != nil {
		slog.Error("ACL check error", "username", username, "topic", topic, "action", action, "error", err)
		return false
	}

	return allowed
}
