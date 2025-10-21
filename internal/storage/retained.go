package storage

import "database/sql"

// RetainedMessage represents a retained MQTT message
type RetainedMessage struct {
	Topic   string
	Payload []byte
	QoS     byte
}

// SaveRetainedMessage stores or updates a retained message (UPSERT)
func (db *DB) SaveRetainedMessage(topic string, payload []byte, qos byte) error {
	_, err := db.Exec(`
		INSERT INTO retained_messages (topic, payload, qos, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(topic) DO UPDATE SET
			payload = excluded.payload,
			qos = excluded.qos,
			created_at = CURRENT_TIMESTAMP
	`, topic, payload, qos)
	return err
}

// DeleteRetainedMessage removes a retained message for a topic
func (db *DB) DeleteRetainedMessage(topic string) error {
	_, err := db.Exec("DELETE FROM retained_messages WHERE topic = ?", topic)
	return err
}

// GetRetainedMessage retrieves a retained message for a specific topic
func (db *DB) GetRetainedMessage(topic string) (*RetainedMessage, error) {
	var msg RetainedMessage
	err := db.QueryRow(
		"SELECT topic, payload, qos FROM retained_messages WHERE topic = ?",
		topic,
	).Scan(&msg.Topic, &msg.Payload, &msg.QoS)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetAllRetainedMessages retrieves all retained messages
func (db *DB) GetAllRetainedMessages() ([]*RetainedMessage, error) {
	rows, err := db.Query("SELECT topic, payload, qos FROM retained_messages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*RetainedMessage
	for rows.Next() {
		var msg RetainedMessage
		if err := rows.Scan(&msg.Topic, &msg.Payload, &msg.QoS); err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}
