package auth

import (
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
)

// MockACLChecker implements the ACLChecker interface for testing
type MockACLChecker struct {
	rules map[string]map[string]bool // username -> topic -> allowed
}

func NewMockACLChecker() *MockACLChecker {
	return &MockACLChecker{
		rules: make(map[string]map[string]bool),
	}
}

func (m *MockACLChecker) AddRule(username, topic, action string, allowed bool) {
	if m.rules[username] == nil {
		m.rules[username] = make(map[string]bool)
	}
	key := topic + ":" + action
	m.rules[username][key] = allowed
}

func (m *MockACLChecker) CheckACL(username, topic, action string) (bool, error) {
	if m.rules[username] == nil {
		return false, nil
	}
	key := topic + ":" + action
	allowed, ok := m.rules[username][key]
	if !ok {
		return false, nil
	}
	return allowed, nil
}

func TestACLHook_ID(t *testing.T) {
	checker := NewMockACLChecker()
	hook := NewACLHook(checker)

	if hook.ID() != "database-acl" {
		t.Errorf("ACLHook.ID() = %v, want database-acl", hook.ID())
	}
}

func TestACLHook_Provides(t *testing.T) {
	checker := NewMockACLChecker()
	hook := NewACLHook(checker)

	tests := []struct {
		name     string
		hookType byte
		want     bool
	}{
		{
			name:     "provides OnACLCheck",
			hookType: mqtt.OnACLCheck,
			want:     true,
		},
		{
			name:     "does not provide OnConnect",
			hookType: mqtt.OnConnect,
			want:     false,
		},
		{
			name:     "does not provide OnDisconnect",
			hookType: mqtt.OnDisconnect,
			want:     false,
		},
		{
			name:     "does not provide OnPublish",
			hookType: mqtt.OnPublish,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hook.Provides(tt.hookType); got != tt.want {
				t.Errorf("ACLHook.Provides(%v) = %v, want %v", tt.hookType, got, tt.want)
			}
		})
	}
}

func TestACLHook_OnACLCheck(t *testing.T) {
	checker := NewMockACLChecker()

	// Set up test rules
	checker.AddRule("testuser", "devices/sensor1/telemetry", "pub", true)
	checker.AddRule("testuser", "devices/sensor1/telemetry", "sub", false)
	checker.AddRule("testuser", "commands/device1", "sub", true)
	checker.AddRule("testuser", "commands/device1", "pub", false)
	checker.AddRule("adminuser", "any/topic", "pub", true)
	checker.AddRule("adminuser", "any/topic", "sub", true)

	hook := NewACLHook(checker)

	tests := []struct {
		name     string
		username string
		topic    string
		write    bool // true = publish, false = subscribe
		want     bool
	}{
		{
			name:     "user can publish to allowed topic",
			username: "testuser",
			topic:    "devices/sensor1/telemetry",
			write:    true,
			want:     true,
		},
		{
			name:     "user cannot subscribe to publish-only topic",
			username: "testuser",
			topic:    "devices/sensor1/telemetry",
			write:    false,
			want:     false,
		},
		{
			name:     "user can subscribe to allowed topic",
			username: "testuser",
			topic:    "commands/device1",
			write:    false,
			want:     true,
		},
		{
			name:     "user cannot publish to subscribe-only topic",
			username: "testuser",
			topic:    "commands/device1",
			write:    true,
			want:     false,
		},
		{
			name:     "admin can publish",
			username: "adminuser",
			topic:    "any/topic",
			write:    true,
			want:     true,
		},
		{
			name:     "admin can subscribe",
			username: "adminuser",
			topic:    "any/topic",
			write:    false,
			want:     true,
		},
		{
			name:     "user denied for unmatched topic publish",
			username: "testuser",
			topic:    "unmatched/topic",
			write:    true,
			want:     false,
		},
		{
			name:     "user denied for unmatched topic subscribe",
			username: "testuser",
			topic:    "unmatched/topic",
			write:    false,
			want:     false,
		},
		{
			name:     "anonymous user publish",
			username: "",
			topic:    "public/topic",
			write:    true,
			want:     false,
		},
		{
			name:     "anonymous user subscribe",
			username: "",
			topic:    "public/topic",
			write:    false,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock client
			cl := &mqtt.Client{
				ID: "test-client",
				Properties: mqtt.ClientProperties{
					Username: []byte(tt.username),
				},
			}

			got := hook.OnACLCheck(cl, tt.topic, tt.write)
			if got != tt.want {
				action := "subscribe"
				if tt.write {
					action = "publish"
				}
				t.Errorf("OnACLCheck(username=%v, topic=%v, action=%v) = %v, want %v",
					tt.username, tt.topic, action, got, tt.want)
			}
		})
	}
}

func TestACLHook_OnACLCheck_Anonymous(t *testing.T) {
	checker := NewMockACLChecker()

	// Set up rules for anonymous user
	// Note: Mock checker uses exact topic:action matching, not wildcards
	checker.AddRule("anonymous", "public/news", "sub", true)
	checker.AddRule("anonymous", "public/announce", "pub", true)

	hook := NewACLHook(checker)

	tests := []struct {
		name  string
		topic string
		write bool
		want  bool
	}{
		{
			name:  "anonymous can subscribe to public topics",
			topic: "public/news",
			write: false,
			want:  true,
		},
		{
			name:  "anonymous can publish to announce",
			topic: "public/announce",
			write: true,
			want:  true,
		},
		{
			name:  "anonymous cannot publish to other public topics",
			topic: "public/news",
			write: true,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a client with no username (anonymous)
			cl := &mqtt.Client{
				ID: "anonymous-client",
				Properties: mqtt.ClientProperties{
					Username: []byte(""),
				},
			}

			got := hook.OnACLCheck(cl, tt.topic, tt.write)
			if got != tt.want {
				action := "subscribe"
				if tt.write {
					action = "publish"
				}
				t.Errorf("OnACLCheck(anonymous, topic=%v, action=%v) = %v, want %v",
					tt.topic, action, got, tt.want)
			}
		})
	}
}
