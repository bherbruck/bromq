package retained

import (
	"fmt"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	"github/bromq-dev/bromq/internal/badgerstore"
)

// MockRetainedStore implements the RetainedStore interface for testing
type MockRetainedStore struct {
	messages map[string]*badgerstore.RetainedMessage
}

func NewMockRetainedStore() *MockRetainedStore {
	return &MockRetainedStore{
		messages: make(map[string]*badgerstore.RetainedMessage),
	}
}

func (m *MockRetainedStore) SaveRetainedMessage(topic string, payload []byte, qos byte) error {
	m.messages[topic] = &badgerstore.RetainedMessage{
		Topic:   topic,
		Payload: payload,
		QoS:     qos,
	}
	return nil
}

func (m *MockRetainedStore) DeleteRetainedMessage(topic string) error {
	if _, exists := m.messages[topic]; !exists {
		return fmt.Errorf("message not found")
	}
	delete(m.messages, topic)
	return nil
}

func (m *MockRetainedStore) GetRetainedMessage(topic string) (*badgerstore.RetainedMessage, error) {
	msg, exists := m.messages[topic]
	if !exists {
		return nil, nil
	}
	return msg, nil
}

func (m *MockRetainedStore) GetAllRetainedMessages() ([]*badgerstore.RetainedMessage, error) {
	messages := make([]*badgerstore.RetainedMessage, 0, len(m.messages))
	for _, msg := range m.messages {
		messages = append(messages, msg)
	}
	return messages, nil
}

func TestRetainedHook_ID(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	if hook.ID() != "retained-persistence" {
		t.Errorf("RetainedHook.ID() = %v, want retained-persistence", hook.ID())
	}
}

func TestRetainedHook_Provides(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	tests := []struct {
		name     string
		hookType byte
		want     bool
	}{
		{
			name:     "provides OnRetainMessage",
			hookType: mqtt.OnRetainMessage,
			want:     true,
		},
		{
			name:     "provides OnRetainedExpired",
			hookType: mqtt.OnRetainedExpired,
			want:     true,
		},
		{
			name:     "provides StoredRetainedMessages",
			hookType: mqtt.StoredRetainedMessages,
			want:     true,
		},
		{
			name:     "does not provide OnConnect",
			hookType: mqtt.OnConnect,
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
				t.Errorf("RetainedHook.Provides(%v) = %v, want %v", tt.hookType, got, tt.want)
			}
		})
	}
}

func TestRetainedHook_OnRetainMessage_Save(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	tests := []struct {
		name    string
		topic   string
		payload []byte
		qos     byte
	}{
		{
			name:    "save simple message",
			topic:   "test/topic",
			payload: []byte("hello"),
			qos:     1,
		},
		{
			name:    "save message with QoS 0",
			topic:   "sensor/temperature",
			payload: []byte("22.5"),
			qos:     0,
		},
		{
			name:    "save message with QoS 2",
			topic:   "critical/alert",
			payload: []byte("alarm!"),
			qos:     2,
		},
		{
			name:    "overwrite existing message",
			topic:   "test/topic",
			payload: []byte("new value"),
			qos:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mqtt.Client{ID: "test-client"}
			pk := packets.Packet{
				TopicName: tt.topic,
				Payload:   tt.payload,
				FixedHeader: packets.FixedHeader{
					Qos: tt.qos,
				},
			}

			hook.OnRetainMessage(client, pk, 1) // r=1 means retain

			if msg, exists := store.messages[tt.topic]; !exists {
				t.Errorf("Expected message for topic %s to be saved", tt.topic)
			} else {
				if string(msg.Payload) != string(tt.payload) {
					t.Errorf("Payload = %s, want %s", msg.Payload, tt.payload)
				}
				if msg.QoS != tt.qos {
					t.Errorf("QoS = %d, want %d", msg.QoS, tt.qos)
				}
			}
		})
	}
}

func TestRetainedHook_OnRetainMessage_Delete(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	// First save a message
	topic := "test/topic"
	store.SaveRetainedMessage(topic, []byte("test"), 1)

	if len(store.messages) != 1 {
		t.Fatalf("Expected 1 message before delete, got %d", len(store.messages))
	}

	// Now delete it (r=-1 means delete)
	client := &mqtt.Client{ID: "test-client"}
	pk := packets.Packet{
		TopicName: topic,
		Payload:   []byte{}, // Empty payload for delete
	}

	hook.OnRetainMessage(client, pk, -1) // r=-1 means delete

	if len(store.messages) != 0 {
		t.Errorf("Expected 0 messages after delete, got %d", len(store.messages))
	}
}

func TestRetainedHook_StoredRetainedMessages(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	// Add some messages to the store
	testMessages := []struct {
		topic   string
		payload string
		qos     byte
	}{
		{"sensor/temp", "22.5", 1},
		{"sensor/humidity", "65", 1},
		{"device/status", "online", 0},
	}

	for _, msg := range testMessages {
		store.SaveRetainedMessage(msg.topic, []byte(msg.payload), msg.qos)
	}

	// Load messages
	messages, err := hook.StoredRetainedMessages()
	if err != nil {
		t.Fatalf("StoredRetainedMessages() returned error: %v", err)
	}

	if len(messages) != len(testMessages) {
		t.Errorf("Expected %d messages, got %d", len(testMessages), len(messages))
	}

	// Verify message format
	for _, msg := range messages {
		if msg.FixedHeader.Type != packets.Publish {
			t.Errorf("Expected message type Publish, got %d", msg.FixedHeader.Type)
		}
		if !msg.FixedHeader.Retain {
			t.Error("Expected Retain flag to be true")
		}
	}
}

func TestRetainedHook_StoredRetainedMessages_Empty(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	messages, err := hook.StoredRetainedMessages()
	if err != nil {
		t.Fatalf("StoredRetainedMessages() returned error: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(messages))
	}
}

func TestRetainedHook_OnRetainedExpired(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	// Add a message
	topic := "expired/topic"
	store.SaveRetainedMessage(topic, []byte("old message"), 1)

	if len(store.messages) != 1 {
		t.Fatalf("Expected 1 message before expiry, got %d", len(store.messages))
	}

	// Expire it
	hook.OnRetainedExpired(topic)

	if len(store.messages) != 0 {
		t.Errorf("Expected 0 messages after expiry, got %d", len(store.messages))
	}
}

func TestRetainedHook_OnRetainMessage_UpdateExisting(t *testing.T) {
	store := NewMockRetainedStore()
	hook := NewRetainedHook(store)

	topic := "test/topic"
	client := &mqtt.Client{ID: "test-client"}

	// Save initial message
	pk1 := packets.Packet{
		TopicName: topic,
		Payload:   []byte("first"),
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
	}
	hook.OnRetainMessage(client, pk1, 1)

	if len(store.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(store.messages))
	}

	// Update with new message
	pk2 := packets.Packet{
		TopicName: topic,
		Payload:   []byte("second"),
		FixedHeader: packets.FixedHeader{
			Qos: 2,
		},
	}
	hook.OnRetainMessage(client, pk2, 1)

	if len(store.messages) != 1 {
		t.Errorf("Expected 1 message after update, got %d", len(store.messages))
	}

	msg := store.messages[topic]
	if string(msg.Payload) != "second" {
		t.Errorf("Payload = %s, want second", msg.Payload)
	}
	if msg.QoS != 2 {
		t.Errorf("QoS = %d, want 2", msg.QoS)
	}
}
