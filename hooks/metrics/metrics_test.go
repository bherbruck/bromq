package metrics

import (
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// MockMetricsRecorder implements the MetricsRecorder interface for testing
type MockMetricsRecorder struct {
	clients          map[string]bool
	messagesReceived map[string]int64
	messagesSent     map[string]int64
	packetsReceived  map[string]int64
	packetsSent      map[string]int64
	bytesReceived    map[string]int64
	bytesSent        map[string]int64
}

func NewMockMetricsRecorder() *MockMetricsRecorder {
	return &MockMetricsRecorder{
		clients:          make(map[string]bool),
		messagesReceived: make(map[string]int64),
		messagesSent:     make(map[string]int64),
		packetsReceived:  make(map[string]int64),
		packetsSent:      make(map[string]int64),
		bytesReceived:    make(map[string]int64),
		bytesSent:        make(map[string]int64),
	}
}

func (m *MockMetricsRecorder) RegisterClient(clientID string) {
	m.clients[clientID] = true
}

func (m *MockMetricsRecorder) UnregisterClient(clientID string) {
	delete(m.clients, clientID)
}

func (m *MockMetricsRecorder) RecordMessageReceived(clientID string, bytes int64) {
	m.messagesReceived[clientID]++
	m.bytesReceived[clientID] += bytes
}

func (m *MockMetricsRecorder) RecordMessageSent(clientID string, bytes int64) {
	m.messagesSent[clientID]++
	m.bytesSent[clientID] += bytes
}

func (m *MockMetricsRecorder) RecordPacketReceived(clientID string, bytes int64) {
	m.packetsReceived[clientID]++
}

func (m *MockMetricsRecorder) RecordPacketSent(clientID string, bytes int64) {
	m.packetsSent[clientID]++
}

func TestMetricsHook_ID(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	if hook.ID() != "metrics-tracker" {
		t.Errorf("MetricsHook.ID() = %v, want metrics-tracker", hook.ID())
	}
}

func TestMetricsHook_Provides(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

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
			name:     "provides OnPacketRead",
			hookType: mqtt.OnPacketRead,
			want:     true,
		},
		{
			name:     "provides OnPacketSent",
			hookType: mqtt.OnPacketSent,
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
				t.Errorf("MetricsHook.Provides(%v) = %v, want %v", tt.hookType, got, tt.want)
			}
		})
	}
}

func TestMetricsHook_OnConnect(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	client := &mqtt.Client{ID: "test-client"}
	pk := packets.Packet{}

	err := hook.OnConnect(client, pk)
	if err != nil {
		t.Errorf("OnConnect() returned error: %v", err)
	}

	if !recorder.clients["test-client"] {
		t.Error("Expected client to be registered")
	}
}

func TestMetricsHook_OnDisconnect(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	clientID := "test-client"
	recorder.RegisterClient(clientID)

	if !recorder.clients[clientID] {
		t.Fatal("Expected client to be registered before disconnect")
	}

	client := &mqtt.Client{ID: clientID}
	hook.OnDisconnect(client, nil, false)

	if recorder.clients[clientID] {
		t.Error("Expected client to be unregistered after disconnect")
	}
}

func TestMetricsHook_OnPacketRead_Publish(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	clientID := "test-client"
	client := &mqtt.Client{ID: clientID}

	// Create a PUBLISH packet (type 3)
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      3, // PUBLISH
			Remaining: 100,
		},
		TopicName: "test/topic",
		Payload:   []byte("test message"),
	}

	resultPk, err := hook.OnPacketRead(client, pk)
	if err != nil {
		t.Errorf("OnPacketRead() returned error: %v", err)
	}

	if resultPk.TopicName != pk.TopicName {
		t.Error("OnPacketRead() should return the same packet")
	}

	// Should increment both packet and message counters for PUBLISH
	if recorder.packetsReceived[clientID] != 1 {
		t.Errorf("packetsReceived = %d, want 1", recorder.packetsReceived[clientID])
	}

	if recorder.messagesReceived[clientID] != 1 {
		t.Errorf("messagesReceived = %d, want 1", recorder.messagesReceived[clientID])
	}
}

func TestMetricsHook_OnPacketRead_NonPublish(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	clientID := "test-client"
	client := &mqtt.Client{ID: clientID}

	// Create a CONNECT packet (type 1)
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      1, // CONNECT
			Remaining: 50,
		},
	}

	_, err := hook.OnPacketRead(client, pk)
	if err != nil {
		t.Errorf("OnPacketRead() returned error: %v", err)
	}

	// Should increment packet counter but not message counter
	if recorder.packetsReceived[clientID] != 1 {
		t.Errorf("packetsReceived = %d, want 1", recorder.packetsReceived[clientID])
	}

	if recorder.messagesReceived[clientID] != 0 {
		t.Errorf("messagesReceived = %d, want 0 (not a PUBLISH)", recorder.messagesReceived[clientID])
	}
}

func TestMetricsHook_OnPacketSent_Publish(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	clientID := "test-client"
	client := &mqtt.Client{ID: clientID}

	// Create a PUBLISH packet (type 3)
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: 3, // PUBLISH
		},
		TopicName: "test/topic",
		Payload:   []byte("test message"),
	}

	packetBytes := []byte{0x30, 0x10, 0x00, 0x09} // Mock packet bytes
	hook.OnPacketSent(client, pk, packetBytes)

	// Should increment both packet and message counters for PUBLISH
	if recorder.packetsSent[clientID] != 1 {
		t.Errorf("packetsSent = %d, want 1", recorder.packetsSent[clientID])
	}

	if recorder.messagesSent[clientID] != 1 {
		t.Errorf("messagesSent = %d, want 1", recorder.messagesSent[clientID])
	}
}

func TestMetricsHook_OnPacketSent_NonPublish(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	clientID := "test-client"
	client := &mqtt.Client{ID: clientID}

	// Create a CONNACK packet (type 2)
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: 2, // CONNACK
		},
	}

	packetBytes := []byte{0x20, 0x02, 0x00, 0x00} // Mock CONNACK
	hook.OnPacketSent(client, pk, packetBytes)

	// Should increment packet counter but not message counter
	if recorder.packetsSent[clientID] != 1 {
		t.Errorf("packetsSent = %d, want 1", recorder.packetsSent[clientID])
	}

	if recorder.messagesSent[clientID] != 0 {
		t.Errorf("messagesSent = %d, want 0 (not a PUBLISH)", recorder.messagesSent[clientID])
	}
}

func TestMetricsHook_MultipleClients(t *testing.T) {
	recorder := NewMockMetricsRecorder()
	hook := NewMetricsHook(recorder)

	// Register multiple clients
	clients := []string{"client-1", "client-2", "client-3"}
	for _, clientID := range clients {
		client := &mqtt.Client{ID: clientID}
		pk := packets.Packet{}
		hook.OnConnect(client, pk)
	}

	if len(recorder.clients) != 3 {
		t.Errorf("Expected 3 clients registered, got %d", len(recorder.clients))
	}

	// Disconnect one client
	client := &mqtt.Client{ID: "client-2"}
	hook.OnDisconnect(client, nil, false)

	if len(recorder.clients) != 2 {
		t.Errorf("Expected 2 clients after disconnect, got %d", len(recorder.clients))
	}

	if recorder.clients["client-2"] {
		t.Error("client-2 should be unregistered")
	}
}
