package storage

import (
	"testing"
)

func TestSaveRetainedMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		topic   string
		payload []byte
		qos     byte
		wantErr bool
	}{
		{
			name:    "save simple message",
			topic:   "sensor/temperature",
			payload: []byte("22.5"),
			qos:     1,
			wantErr: false,
		},
		{
			name:    "save message with QoS 0",
			topic:   "device/status",
			payload: []byte("online"),
			qos:     0,
			wantErr: false,
		},
		{
			name:    "save message with QoS 2",
			topic:   "alert/critical",
			payload: []byte("fire alarm"),
			qos:     2,
			wantErr: false,
		},
		{
			name:    "save message with empty payload",
			topic:   "test/empty",
			payload: []byte{},
			qos:     0,
			wantErr: false,
		},
		{
			name:    "save message with large payload",
			topic:   "data/bulk",
			payload: make([]byte, 10000),
			qos:     1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.SaveRetainedMessage(tt.topic, tt.payload, tt.qos)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SaveRetainedMessage() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("SaveRetainedMessage() unexpected error: %v", err)
			}

			// Verify the message was saved
			msg, err := db.GetRetainedMessage(tt.topic)
			if err != nil {
				t.Fatalf("GetRetainedMessage() failed: %v", err)
			}

			if msg == nil {
				t.Fatal("GetRetainedMessage() returned nil")
			}

			if msg.Topic != tt.topic {
				t.Errorf("Topic = %v, want %v", msg.Topic, tt.topic)
			}

			if string(msg.Payload) != string(tt.payload) {
				t.Errorf("Payload = %v, want %v", msg.Payload, tt.payload)
			}

			if msg.QoS != tt.qos {
				t.Errorf("QoS = %v, want %v", msg.QoS, tt.qos)
			}
		})
	}
}

func TestSaveRetainedMessage_Upsert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	topic := "test/upsert"

	// Save initial message
	err := db.SaveRetainedMessage(topic, []byte("first value"), 1)
	if err != nil {
		t.Fatalf("Failed to save initial message: %v", err)
	}

	msg1, _ := db.GetRetainedMessage(topic)
	initialTime := msg1.CreatedAt

	// Update the message (upsert)
	err = db.SaveRetainedMessage(topic, []byte("second value"), 2)
	if err != nil {
		t.Fatalf("Failed to upsert message: %v", err)
	}

	// Verify the message was updated, not duplicated
	allMessages, err := db.GetAllRetainedMessages()
	if err != nil {
		t.Fatalf("GetAllRetainedMessages() failed: %v", err)
	}

	count := 0
	for _, msg := range allMessages {
		if msg.Topic == topic {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected 1 message for topic %s, found %d", topic, count)
	}

	// Verify the content was updated
	msg2, err := db.GetRetainedMessage(topic)
	if err != nil {
		t.Fatalf("GetRetainedMessage() failed: %v", err)
	}

	if string(msg2.Payload) != "second value" {
		t.Errorf("Payload = %s, want 'second value'", msg2.Payload)
	}

	if msg2.QoS != 2 {
		t.Errorf("QoS = %d, want 2", msg2.QoS)
	}

	// CreatedAt should be updated
	if !msg2.CreatedAt.After(initialTime) {
		t.Errorf("CreatedAt should be updated during upsert")
	}
}

func TestGetRetainedMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Save test messages
	db.SaveRetainedMessage("test/topic", []byte("test payload"), 1)

	tests := []struct {
		name     string
		topic    string
		wantNil  bool
		wantErr  bool
	}{
		{
			name:    "get existing message",
			topic:   "test/topic",
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "get non-existent message",
			topic:   "nonexistent/topic",
			wantNil: true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := db.GetRetainedMessage(tt.topic)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetRetainedMessage() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetRetainedMessage() unexpected error: %v", err)
			}

			if tt.wantNil {
				if msg != nil {
					t.Errorf("GetRetainedMessage() expected nil but got message")
				}
				return
			}

			if msg == nil {
				t.Fatal("GetRetainedMessage() expected message but got nil")
			}

			if msg.Topic != tt.topic {
				t.Errorf("Topic = %v, want %v", msg.Topic, tt.topic)
			}
		})
	}
}

func TestDeleteRetainedMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		setup   func() string // Returns topic to delete
		wantErr bool
	}{
		{
			name: "delete existing message",
			setup: func() string {
				topic := "delete/test"
				db.SaveRetainedMessage(topic, []byte("to be deleted"), 1)
				return topic
			},
			wantErr: false,
		},
		{
			name: "delete non-existent message",
			setup: func() string {
				return "nonexistent/topic"
			},
			wantErr: false, // GORM doesn't error on 0 rows affected for Delete
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := tt.setup()
			err := db.DeleteRetainedMessage(topic)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteRetainedMessage() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteRetainedMessage() unexpected error: %v", err)
			}

			// Verify message is deleted
			msg, err := db.GetRetainedMessage(topic)
			if err != nil {
				t.Fatalf("GetRetainedMessage() failed: %v", err)
			}

			if msg != nil {
				t.Errorf("DeleteRetainedMessage() message still exists")
			}
		})
	}
}

func TestGetAllRetainedMessages(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Start with empty database
	messages, err := db.GetAllRetainedMessages()
	if err != nil {
		t.Fatalf("GetAllRetainedMessages() failed on empty db: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages in empty database, got %d", len(messages))
	}

	// Add test messages
	testMessages := []struct {
		topic   string
		payload string
		qos     byte
	}{
		{"sensor/temp", "22.5", 1},
		{"sensor/humidity", "65", 1},
		{"device/status", "online", 0},
		{"alert/fire", "active", 2},
	}

	for _, tm := range testMessages {
		err := db.SaveRetainedMessage(tm.topic, []byte(tm.payload), tm.qos)
		if err != nil {
			t.Fatalf("Failed to save test message: %v", err)
		}
	}

	// Get all messages
	messages, err = db.GetAllRetainedMessages()
	if err != nil {
		t.Fatalf("GetAllRetainedMessages() failed: %v", err)
	}

	if len(messages) != len(testMessages) {
		t.Errorf("Expected %d messages, got %d", len(testMessages), len(messages))
	}

	// Verify all test messages are present
	topicMap := make(map[string]*RetainedMessage)
	for _, msg := range messages {
		topicMap[msg.Topic] = msg
	}

	for _, tm := range testMessages {
		msg, exists := topicMap[tm.topic]
		if !exists {
			t.Errorf("Message for topic %s not found", tm.topic)
			continue
		}

		if string(msg.Payload) != tm.payload {
			t.Errorf("Topic %s: payload = %s, want %s", tm.topic, msg.Payload, tm.payload)
		}

		if msg.QoS != tm.qos {
			t.Errorf("Topic %s: QoS = %d, want %d", tm.topic, msg.QoS, tm.qos)
		}
	}
}

func TestRetainedMessage_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	topic := "lifecycle/test"
	payload1 := []byte("first message")
	payload2 := []byte("updated message")

	// 1. Save initial message
	err := db.SaveRetainedMessage(topic, payload1, 1)
	if err != nil {
		t.Fatalf("Failed to save initial message: %v", err)
	}

	// 2. Retrieve and verify
	msg, err := db.GetRetainedMessage(topic)
	if err != nil {
		t.Fatalf("Failed to get message: %v", err)
	}
	if msg == nil {
		t.Fatal("Message is nil")
	}
	if string(msg.Payload) != string(payload1) {
		t.Errorf("Initial payload = %s, want %s", msg.Payload, payload1)
	}

	// 3. Update message
	err = db.SaveRetainedMessage(topic, payload2, 2)
	if err != nil {
		t.Fatalf("Failed to update message: %v", err)
	}

	// 4. Verify update
	msg, err = db.GetRetainedMessage(topic)
	if err != nil {
		t.Fatalf("Failed to get updated message: %v", err)
	}
	if string(msg.Payload) != string(payload2) {
		t.Errorf("Updated payload = %s, want %s", msg.Payload, payload2)
	}
	if msg.QoS != 2 {
		t.Errorf("Updated QoS = %d, want 2", msg.QoS)
	}

	// 5. Delete message
	err = db.DeleteRetainedMessage(topic)
	if err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	// 6. Verify deletion
	msg, err = db.GetRetainedMessage(topic)
	if err != nil {
		t.Fatalf("Failed to check deleted message: %v", err)
	}
	if msg != nil {
		t.Errorf("Message should be deleted but still exists")
	}
}

func TestRetainedMessage_MultipleTopics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	topics := []string{
		"home/living-room/temp",
		"home/bedroom/temp",
		"home/kitchen/humidity",
		"home/+/temp", // Wildcard pattern (stored as literal)
		"device/#",    // Wildcard pattern (stored as literal)
	}

	// Save messages for all topics
	for i, topic := range topics {
		payload := []byte(topic + " data")
		qos := byte(i % 3)
		err := db.SaveRetainedMessage(topic, payload, qos)
		if err != nil {
			t.Fatalf("Failed to save message for topic %s: %v", topic, i)
		}
	}

	// Verify all messages are stored
	messages, err := db.GetAllRetainedMessages()
	if err != nil {
		t.Fatalf("GetAllRetainedMessages() failed: %v", err)
	}

	if len(messages) != len(topics) {
		t.Errorf("Expected %d messages, got %d", len(topics), len(messages))
	}

	// Verify each topic can be retrieved individually
	for _, topic := range topics {
		msg, err := db.GetRetainedMessage(topic)
		if err != nil {
			t.Errorf("GetRetainedMessage(%s) failed: %v", topic, err)
		}
		if msg == nil {
			t.Errorf("GetRetainedMessage(%s) returned nil", topic)
		}
		if msg != nil && msg.Topic != topic {
			t.Errorf("GetRetainedMessage(%s) returned wrong topic: %s", topic, msg.Topic)
		}
	}
}

func TestRetainedMessage_QoSLevels(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name string
		qos  byte
	}{
		{"QoS 0 - At most once", 0},
		{"QoS 1 - At least once", 1},
		{"QoS 2 - Exactly once", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := "qos/test/" + tt.name
			payload := []byte("QoS test payload")

			err := db.SaveRetainedMessage(topic, payload, tt.qos)
			if err != nil {
				t.Fatalf("SaveRetainedMessage() failed: %v", err)
			}

			msg, err := db.GetRetainedMessage(topic)
			if err != nil {
				t.Fatalf("GetRetainedMessage() failed: %v", err)
			}

			if msg == nil {
				t.Fatal("Message is nil")
			}

			if msg.QoS != tt.qos {
				t.Errorf("QoS = %d, want %d", msg.QoS, tt.qos)
			}
		})
	}
}

func TestRetainedMessage_EmptyPayload(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	topic := "empty/payload"

	// Save message with empty payload (MQTT allows this)
	err := db.SaveRetainedMessage(topic, []byte{}, 1)
	if err != nil {
		t.Fatalf("SaveRetainedMessage() with empty payload failed: %v", err)
	}

	// Retrieve and verify
	msg, err := db.GetRetainedMessage(topic)
	if err != nil {
		t.Fatalf("GetRetainedMessage() failed: %v", err)
	}

	if msg == nil {
		t.Fatal("Message is nil")
	}

	if len(msg.Payload) != 0 {
		t.Errorf("Payload length = %d, want 0", len(msg.Payload))
	}
}

func TestRetainedMessage_ConcurrentUpserts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	topic := "concurrent/test"

	// Simulate rapid upserts (like a sensor sending frequent updates)
	for i := 0; i < 10; i++ {
		payload := []byte("update " + string(rune('0'+i)))
		err := db.SaveRetainedMessage(topic, payload, byte(i%3))
		if err != nil {
			t.Fatalf("Upsert %d failed: %v", i, err)
		}
	}

	// Should only have one message for the topic
	messages, err := db.GetAllRetainedMessages()
	if err != nil {
		t.Fatalf("GetAllRetainedMessages() failed: %v", err)
	}

	count := 0
	for _, msg := range messages {
		if msg.Topic == topic {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected 1 message after multiple upserts, got %d", count)
	}
}
