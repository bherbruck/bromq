package badgerstore

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// RetainedMessage represents a retained MQTT message in BadgerDB
type RetainedMessage struct {
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	QoS       byte      `json:"qos"`
	CreatedAt time.Time `json:"created_at"`
}

// retainedMessageData represents the JSON structure stored in BadgerDB
type retainedMessageData struct {
	Topic   string `json:"topic"`
	Payload []byte `json:"payload"`
	QoS     byte   `json:"qos"`
}

// SaveRetainedMessage stores or updates a retained message (topic is the key)
func (b *BadgerStore) SaveRetainedMessage(topic string, payload []byte, qos byte) error {
	msg := retainedMessageData{
		Topic:   topic,
		Payload: payload,
		QoS:     qos,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal retained message: %w", err)
	}

	// Use topic as key with "retained:" prefix
	key := fmt.Sprintf("retained:%s", topic)
	return b.Set(key, data, 0) // No TTL - retained messages persist indefinitely
}

// DeleteRetainedMessage removes a retained message for a topic
func (b *BadgerStore) DeleteRetainedMessage(topic string) error {
	key := fmt.Sprintf("retained:%s", topic)
	return b.Delete(key)
}

// GetRetainedMessage retrieves a retained message for a specific topic
func (b *BadgerStore) GetRetainedMessage(topic string) (*RetainedMessage, error) {
	key := fmt.Sprintf("retained:%s", topic)
	data, err := b.Get(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil // Not found
	}

	var msgData retainedMessageData
	if err := json.Unmarshal(data, &msgData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal retained message: %w", err)
	}

	// Convert to RetainedMessage
	return &RetainedMessage{
		Topic:     msgData.Topic,
		Payload:   msgData.Payload,
		QoS:       msgData.QoS,
		CreatedAt: time.Now(), // BadgerDB doesn't track created_at, use current time
	}, nil
}

// GetAllRetainedMessages retrieves all retained messages
func (b *BadgerStore) GetAllRetainedMessages() ([]*RetainedMessage, error) {
	var messages []*RetainedMessage

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("retained:")

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			value, err := it.Item().ValueCopy(nil)
			if err != nil {
				return err
			}

			var msgData retainedMessageData
			if err := json.Unmarshal(value, &msgData); err != nil {
				return fmt.Errorf("failed to unmarshal retained message: %w", err)
			}

			// Convert to RetainedMessage
			messages = append(messages, &RetainedMessage{
				Topic:     msgData.Topic,
				Payload:   msgData.Payload,
				QoS:       msgData.QoS,
				CreatedAt: time.Now(), // BadgerDB doesn't track created_at
			})
		}
		return nil
	})

	return messages, err
}
