package retained

import (
	"bytes"
	"log"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"

	dbstorage "github/bherbruck/mqtt-server/internal/storage"
)

// RetainedStore interface for storing retained messages
type RetainedStore interface {
	SaveRetainedMessage(topic string, payload []byte, qos byte) error
	DeleteRetainedMessage(topic string) error
	GetAllRetainedMessages() ([]*dbstorage.RetainedMessage, error)
}

// RetainedHook implements MQTT hook for persisting retained messages
type RetainedHook struct {
	mqtt.HookBase
	store RetainedStore
}

// NewRetainedHook creates a new retained message persistence hook
func NewRetainedHook(store RetainedStore) *RetainedHook {
	return &RetainedHook{
		store: store,
	}
}

// ID returns the hook identifier
func (h *RetainedHook) ID() string {
	return "retained-persistence"
}

// Provides indicates which hook methods this hook provides
func (h *RetainedHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnRetainMessage,
		mqtt.OnRetainedExpired,
		mqtt.StoredRetainedMessages,
	}, []byte{b})
}

// OnRetainMessage is called when the server needs to store a retained message
func (h *RetainedHook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	topic := pk.TopicName

	// r == -1 means delete the retained message (empty payload)
	if r == -1 {
		if err := h.store.DeleteRetainedMessage(topic); err != nil {
			log.Printf("Failed to delete retained message for topic %s: %v", topic, err)
		}
		return
	}

	// Save retained message (upsert)
	qos := pk.FixedHeader.Qos
	if err := h.store.SaveRetainedMessage(topic, pk.Payload, qos); err != nil {
		log.Printf("Failed to save retained message for topic %s: %v", topic, err)
	}
}

// StoredRetainedMessages returns all stored retained messages from the database
// This is called by mochi-mqtt on startup to load retained messages into memory
func (h *RetainedHook) StoredRetainedMessages() ([]storage.Message, error) {
	dbMessages, err := h.store.GetAllRetainedMessages()
	if err != nil {
		log.Printf("Failed to load retained messages from database: %v", err)
		return nil, err
	}

	messages := make([]storage.Message, 0, len(dbMessages))
	for _, msg := range dbMessages {
		messages = append(messages, storage.Message{
			ID:        retainedKey(msg.Topic),
			T:         storage.RetainedKey,
			TopicName: msg.Topic,
			Payload:   msg.Payload,
			FixedHeader: packets.FixedHeader{
				Type:   packets.Publish,
				Retain: true,
				Qos:    msg.QoS,
			},
			Created: time.Now().Unix(),
		})
	}

	log.Printf("Loaded %d retained messages from database", len(messages))
	return messages, nil
}

// OnRetainedExpired is called when a retained message expires
func (h *RetainedHook) OnRetainedExpired(filter string) {
	if err := h.store.DeleteRetainedMessage(filter); err != nil {
		log.Printf("Failed to delete expired retained message for filter %s: %v", filter, err)
	}
}

// retainedKey generates a unique key for a retained message
func retainedKey(topic string) string {
	return storage.RetainedKey + ":" + topic
}
