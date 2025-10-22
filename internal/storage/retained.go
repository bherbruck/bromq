package storage

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveRetainedMessage stores or updates a retained message (UPSERT)
func (db *DB) SaveRetainedMessage(topic string, payload []byte, qos byte) error {
	msg := RetainedMessage{
		Topic:   topic,
		Payload: payload,
		QoS:     qos,
	}

	// Use UPSERT (INSERT OR REPLACE for SQLite)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "topic"}},
		DoUpdates: clause.AssignmentColumns([]string{"payload", "qos", "created_at"}),
	}).Create(&msg).Error

	return err
}

// DeleteRetainedMessage removes a retained message for a topic
func (db *DB) DeleteRetainedMessage(topic string) error {
	return db.Where("topic = ?", topic).Delete(&RetainedMessage{}).Error
}

// GetRetainedMessage retrieves a retained message for a specific topic
func (db *DB) GetRetainedMessage(topic string) (*RetainedMessage, error) {
	var msg RetainedMessage
	err := db.Where("topic = ?", topic).First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetAllRetainedMessages retrieves all retained messages
func (db *DB) GetAllRetainedMessages() ([]*RetainedMessage, error) {
	var messages []*RetainedMessage
	err := db.Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}
