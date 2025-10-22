package tracking

import (
	"fmt"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// MockClientTracker implements the ClientTracker interface for testing
type MockClientTracker struct {
	clients map[string]*MockClient // clientID -> client
	users   map[string]uint        // username -> userID
}

type MockClient struct {
	ClientID   string
	MQTTUserID uint
	IsActive   bool
}

type MockUser struct {
	ID       uint
	Username string
}

func (m *MockUser) GetID() uint {
	return m.ID
}

func NewMockClientTracker() *MockClientTracker {
	return &MockClientTracker{
		clients: make(map[string]*MockClient),
		users:   make(map[string]uint),
	}
}

func (m *MockClientTracker) AddUser(username string, userID uint) {
	m.users[username] = userID
}

func (m *MockClientTracker) UpsertMQTTClientInterface(clientID string, mqttUserID uint, metadata interface{}) (interface{}, error) {
	if client, exists := m.clients[clientID]; exists {
		// Update existing
		client.MQTTUserID = mqttUserID
		client.IsActive = true
		return client, nil
	}
	// Create new
	client := &MockClient{
		ClientID:   clientID,
		MQTTUserID: mqttUserID,
		IsActive:   true,
	}
	m.clients[clientID] = client
	return client, nil
}

func (m *MockClientTracker) MarkMQTTClientInactive(clientID string) error {
	if client, exists := m.clients[clientID]; exists {
		client.IsActive = false
		return nil
	}
	return fmt.Errorf("client not found")
}

func (m *MockClientTracker) GetMQTTUserByUsernameInterface(username string) (interface{}, error) {
	if userID, exists := m.users[username]; exists {
		return &MockUser{ID: userID, Username: username}, nil
	}
	return nil, fmt.Errorf("user not found")
}

func TestTrackingHook_ID(t *testing.T) {
	tracker := NewMockClientTracker()
	hook := NewTrackingHook(tracker)

	if hook.ID() != "client-tracking" {
		t.Errorf("TrackingHook.ID() = %v, want client-tracking", hook.ID())
	}
}

func TestTrackingHook_Provides(t *testing.T) {
	tracker := NewMockClientTracker()
	hook := NewTrackingHook(tracker)

	tests := []struct {
		name     string
		hookType byte
		want     bool
	}{
		{
			name:     "provides OnConnect",
			hookType: mqtt.OnConnect,
			want:     true,
		},
		{
			name:     "provides OnDisconnect",
			hookType: mqtt.OnDisconnect,
			want:     true,
		},
		{
			name:     "does not provide OnConnectAuthenticate",
			hookType: mqtt.OnConnectAuthenticate,
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
				t.Errorf("TrackingHook.Provides(%v) = %v, want %v", tt.hookType, got, tt.want)
			}
		})
	}
}

func TestTrackingHook_OnConnect(t *testing.T) {
	tests := []struct {
		name           string
		clientID       string
		username       string
		setupUsers     map[string]uint
		expectTracked  bool
		expectClientID string
	}{
		{
			name:           "track authenticated user",
			clientID:       "client-001",
			username:       "testuser",
			setupUsers:     map[string]uint{"testuser": 1},
			expectTracked:  true,
			expectClientID: "client-001",
		},
		{
			name:          "skip anonymous connection",
			clientID:      "client-anon",
			username:      "",
			setupUsers:    map[string]uint{},
			expectTracked: false,
		},
		{
			name:          "skip non-existent user",
			clientID:      "client-404",
			username:      "nonexistent",
			setupUsers:    map[string]uint{},
			expectTracked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewMockClientTracker()
			for username, userID := range tt.setupUsers {
				tracker.AddUser(username, userID)
			}
			hook := NewTrackingHook(tracker)

			client := &mqtt.Client{ID: tt.clientID}
			pk := packets.Packet{
				Connect: packets.ConnectParams{
					Username: []byte(tt.username),
				},
			}

			err := hook.OnConnect(client, pk)
			if err != nil {
				t.Errorf("OnConnect() returned error: %v", err)
			}

			if tt.expectTracked {
				if _, exists := tracker.clients[tt.expectClientID]; !exists {
					t.Errorf("Expected client %s to be tracked", tt.expectClientID)
				}
				if !tracker.clients[tt.expectClientID].IsActive {
					t.Errorf("Expected client %s to be active", tt.expectClientID)
				}
			} else {
				if _, exists := tracker.clients[tt.clientID]; exists {
					t.Errorf("Did not expect client %s to be tracked", tt.clientID)
				}
			}
		})
	}
}

func TestTrackingHook_OnConnect_UpdateExisting(t *testing.T) {
	tracker := NewMockClientTracker()
	tracker.AddUser("testuser", 1)
	hook := NewTrackingHook(tracker)

	client := &mqtt.Client{ID: "client-001"}
	pk := packets.Packet{
		Connect: packets.ConnectParams{
			Username: []byte("testuser"),
		},
	}

	// First connect
	err := hook.OnConnect(client, pk)
	if err != nil {
		t.Fatalf("First OnConnect() returned error: %v", err)
	}

	if len(tracker.clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(tracker.clients))
	}

	// Mark inactive
	tracker.MarkMQTTClientInactive("client-001")
	if tracker.clients["client-001"].IsActive {
		t.Error("Expected client to be inactive")
	}

	// Second connect (should update existing)
	err = hook.OnConnect(client, pk)
	if err != nil {
		t.Fatalf("Second OnConnect() returned error: %v", err)
	}

	if len(tracker.clients) != 1 {
		t.Errorf("Expected 1 client after reconnect, got %d", len(tracker.clients))
	}

	if !tracker.clients["client-001"].IsActive {
		t.Error("Expected client to be active after reconnect")
	}
}

func TestTrackingHook_OnDisconnect(t *testing.T) {
	tracker := NewMockClientTracker()
	tracker.AddUser("testuser", 1)
	hook := NewTrackingHook(tracker)

	client := &mqtt.Client{ID: "client-001"}
	pk := packets.Packet{
		Connect: packets.ConnectParams{
			Username: []byte("testuser"),
		},
	}

	// Connect first
	hook.OnConnect(client, pk)

	if !tracker.clients["client-001"].IsActive {
		t.Fatal("Expected client to be active after connect")
	}

	// Disconnect
	hook.OnDisconnect(client, nil, false)

	if tracker.clients["client-001"].IsActive {
		t.Error("Expected client to be inactive after disconnect")
	}
}

func TestTrackingHook_OnDisconnect_NonExistent(t *testing.T) {
	tracker := NewMockClientTracker()
	hook := NewTrackingHook(tracker)

	client := &mqtt.Client{ID: "nonexistent"}

	// Should not panic or error when disconnecting non-tracked client
	hook.OnDisconnect(client, nil, false)
}
