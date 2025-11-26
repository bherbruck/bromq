package badgerstore

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestSaveScriptLog(t *testing.T) {
	store := OpenInMemory(t)

	context := map[string]interface{}{
		"client_id": "test-client",
		"topic":     "test/topic",
	}

	err := store.SaveScriptLog(1, "on_publish", "info", "Test message", context, 15)
	if err != nil {
		t.Fatalf("Failed to save script log: %v", err)
	}

	// Verify log was saved
	logs, total, err := store.ListScriptLogs(1, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to list script logs: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected 1 log, got %d", total)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 log in results, got %d", len(logs))
	}

	log := logs[0]
	if log.ScriptID != 1 {
		t.Errorf("Expected ScriptID 1, got %d", log.ScriptID)
	}
	if log.Type != "on_publish" {
		t.Errorf("Expected Type 'on_publish', got %s", log.Type)
	}
	if log.Level != "info" {
		t.Errorf("Expected Level 'info', got %s", log.Level)
	}
	if log.Message != "Test message" {
		t.Errorf("Expected Message 'Test message', got %s", log.Message)
	}
	if log.ExecutionTimeMs != 15 {
		t.Errorf("Expected ExecutionTimeMs 15, got %d", log.ExecutionTimeMs)
	}
	if log.Context["client_id"] != "test-client" {
		t.Errorf("Expected Context client_id 'test-client', got %v", log.Context["client_id"])
	}
}

func TestListScriptLogs_Pagination(t *testing.T) {
	store := OpenInMemory(t)

	// Create 25 logs with slight delay to ensure unique timestamps
	for i := 0; i < 25; i++ {
		err := store.SaveScriptLog(1, "on_publish", "info", "Test message", nil, 10)
		if err != nil {
			t.Fatalf("Failed to save log %d: %v", i, err)
		}
		time.Sleep(1 * time.Millisecond) // Ensure unique timestamps
	}

	// Test first page
	logs, total, err := store.ListScriptLogs(1, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to list logs (page 1): %v", err)
	}
	if total != 25 {
		t.Errorf("Expected total 25, got %d", total)
	}
	if len(logs) != 10 {
		t.Errorf("Expected 10 logs on page 1, got %d", len(logs))
	}

	// Test second page
	logs, total, err = store.ListScriptLogs(1, 2, 10, "")
	if err != nil {
		t.Fatalf("Failed to list logs (page 2): %v", err)
	}
	if total != 25 {
		t.Errorf("Expected total 25, got %d", total)
	}
	if len(logs) != 10 {
		t.Errorf("Expected 10 logs on page 2, got %d", len(logs))
	}

	// Test third page (partial)
	logs, total, err = store.ListScriptLogs(1, 3, 10, "")
	if err != nil {
		t.Fatalf("Failed to list logs (page 3): %v", err)
	}
	if total != 25 {
		t.Errorf("Expected total 25, got %d", total)
	}
	if len(logs) != 5 {
		t.Errorf("Expected 5 logs on page 3, got %d", len(logs))
	}

	// Test beyond last page
	logs, total, err = store.ListScriptLogs(1, 4, 10, "")
	if err != nil {
		t.Fatalf("Failed to list logs (page 4): %v", err)
	}
	if total != 25 {
		t.Errorf("Expected total 25, got %d", total)
	}
	if len(logs) != 0 {
		t.Errorf("Expected 0 logs on page 4, got %d", len(logs))
	}
}

func TestListScriptLogs_LevelFilter(t *testing.T) {
	store := OpenInMemory(t)

	// Create logs with different levels
	levels := []string{"debug", "info", "info", "warn", "error", "error", "error"}
	for i, level := range levels {
		err := store.SaveScriptLog(1, "on_publish", level, "Test message", nil, 10)
		if err != nil {
			t.Fatalf("Failed to save log %d: %v", i, err)
		}
		time.Sleep(1 * time.Millisecond)
	}

	// Test no filter
	logs, total, err := store.ListScriptLogs(1, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list logs (no filter): %v", err)
	}
	if total != 7 {
		t.Errorf("Expected total 7, got %d", total)
	}

	// Test info filter
	logs, total, err = store.ListScriptLogs(1, 1, 100, "info")
	if err != nil {
		t.Fatalf("Failed to list logs (info filter): %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 info logs, got %d", total)
	}
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs in results, got %d", len(logs))
	}

	// Test error filter
	logs, total, err = store.ListScriptLogs(1, 1, 100, "error")
	if err != nil {
		t.Fatalf("Failed to list logs (error filter): %v", err)
	}
	if total != 3 {
		t.Errorf("Expected 3 error logs, got %d", total)
	}
}

func TestListScriptLogs_SortOrder(t *testing.T) {
	store := OpenInMemory(t)

	// Create logs with identifiable messages
	for i := 1; i <= 5; i++ {
		err := store.SaveScriptLog(1, "on_publish", "info", "Message "+string(rune('0'+i)), nil, 10)
		if err != nil {
			t.Fatalf("Failed to save log %d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond) // Ensure distinct timestamps
	}

	// Verify logs are returned newest first (DESC)
	logs, _, err := store.ListScriptLogs(1, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to list logs: %v", err)
	}

	if len(logs) != 5 {
		t.Fatalf("Expected 5 logs, got %d", len(logs))
	}

	// Newest should be first (Message 5)
	if logs[0].Message != "Message 5" {
		t.Errorf("Expected first message 'Message 5', got %s", logs[0].Message)
	}

	// Oldest should be last (Message 1)
	if logs[4].Message != "Message 1" {
		t.Errorf("Expected last message 'Message 1', got %s", logs[4].Message)
	}

	// Verify timestamps are descending
	for i := 0; i < len(logs)-1; i++ {
		if logs[i].CreatedAt.Before(logs[i+1].CreatedAt) {
			t.Errorf("Logs not in descending order at index %d", i)
		}
	}
}

func TestListScriptLogs_MultipleScripts(t *testing.T) {
	store := OpenInMemory(t)

	// Create logs for different scripts
	err := store.SaveScriptLog(1, "on_publish", "info", "Script 1 log", nil, 10)
	if err != nil {
		t.Fatalf("Failed to save log for script 1: %v", err)
	}

	err = store.SaveScriptLog(2, "on_connect", "warn", "Script 2 log", nil, 20)
	if err != nil {
		t.Fatalf("Failed to save log for script 2: %v", err)
	}

	err = store.SaveScriptLog(1, "on_disconnect", "error", "Another script 1 log", nil, 30)
	if err != nil {
		t.Fatalf("Failed to save another log for script 1: %v", err)
	}

	// List logs for script 1
	logs, total, err := store.ListScriptLogs(1, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list logs for script 1: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 logs for script 1, got %d", total)
	}

	// List logs for script 2
	logs, total, err = store.ListScriptLogs(2, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list logs for script 2: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 log for script 2, got %d", total)
	}
	if logs[0].Message != "Script 2 log" {
		t.Errorf("Expected 'Script 2 log', got %s", logs[0].Message)
	}
}

func TestClearScriptLogs(t *testing.T) {
	store := OpenInMemory(t)

	// Create logs for two scripts
	for i := 0; i < 5; i++ {
		err := store.SaveScriptLog(1, "on_publish", "info", "Script 1 log", nil, 10)
		if err != nil {
			t.Fatalf("Failed to save log for script 1: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
	}

	for i := 0; i < 3; i++ {
		err := store.SaveScriptLog(2, "on_publish", "info", "Script 2 log", nil, 10)
		if err != nil {
			t.Fatalf("Failed to save log for script 2: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
	}

	// Clear logs for script 1
	err := store.ClearScriptLogs(1)
	if err != nil {
		t.Fatalf("Failed to clear logs for script 1: %v", err)
	}

	// Verify script 1 logs are gone
	_, total, err := store.ListScriptLogs(1, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list logs for script 1: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected 0 logs for script 1, got %d", total)
	}

	// Verify script 2 logs still exist
	_, total, err = store.ListScriptLogs(2, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list logs for script 2: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected 3 logs for script 2, got %d", total)
	}
}

func TestClearScriptLogsBefore(t *testing.T) {
	store := OpenInMemory(t)

	now := time.Now()

	// Create old logs (2 hours ago) with proper timestamp-based IDs
	oldTime := now.Add(-2 * time.Hour)
	for i := 0; i < 3; i++ {
		// Create entry with old timestamp
		oldTimestamp := oldTime.Add(time.Duration(i) * time.Millisecond)
		id := fmt.Sprintf("%d", oldTimestamp.UnixNano())

		entry := ScriptLogEntry{
			ID:              id,
			ScriptID:        1,
			Type:            "on_publish",
			Level:           "info",
			Message:         "Old log",
			ExecutionTimeMs: 10,
			CreatedAt:       oldTimestamp,
		}

		// Save with timestamp-based key
		key := fmt.Sprintf("log:1:%s", id)
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("Failed to marshal old entry: %v", err)
		}
		if err := store.Set(key, data, 0); err != nil {
			t.Fatalf("Failed to save old log: %v", err)
		}
	}

	// Create recent logs (now)
	time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps
	for i := 0; i < 2; i++ {
		err := store.SaveScriptLog(1, "on_publish", "info", "Recent log", nil, 10)
		if err != nil {
			t.Fatalf("Failed to save recent log: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Clear logs older than 1 hour
	cutoff := now.Add(-1 * time.Hour)
	err := store.ClearScriptLogsBefore(1, cutoff)
	if err != nil {
		t.Fatalf("Failed to clear old logs: %v", err)
	}

	// Verify recent logs still exist
	_, total, err := store.ListScriptLogs(1, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list logs: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 recent logs remaining, got %d", total)
	}
}

func TestGetScriptLogStats(t *testing.T) {
	store := OpenInMemory(t)

	// Create logs with various levels
	levels := map[string]int{
		"debug": 2,
		"info":  5,
		"warn":  3,
		"error": 1,
	}

	for level, count := range levels {
		for i := 0; i < count; i++ {
			err := store.SaveScriptLog(1, "on_publish", level, "Test message", nil, 10)
			if err != nil {
				t.Fatalf("Failed to save %s log: %v", level, err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Get stats
	stats, err := store.GetScriptLogStats(1)
	if err != nil {
		t.Fatalf("Failed to get log stats: %v", err)
	}

	// Verify stats
	for level, expectedCount := range levels {
		if stats[level] != int64(expectedCount) {
			t.Errorf("Expected %d %s logs, got %d", expectedCount, level, stats[level])
		}
	}

	expectedTotal := int64(2 + 5 + 3 + 1)
	if stats["total"] != expectedTotal {
		t.Errorf("Expected total %d, got %d", expectedTotal, stats["total"])
	}
}

func TestCountScriptLogs(t *testing.T) {
	store := OpenInMemory(t)

	// Initially should be 0
	count, err := store.CountScriptLogs(1)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 logs initially, got %d", count)
	}

	// Add logs
	for i := 0; i < 7; i++ {
		err := store.SaveScriptLog(1, "on_publish", "info", "Test", nil, 10)
		if err != nil {
			t.Fatalf("Failed to save log: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
	}

	// Count should be 7
	count, err = store.CountScriptLogs(1)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 7 {
		t.Errorf("Expected 7 logs, got %d", count)
	}
}
