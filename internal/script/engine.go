package script

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"

	"github/bherbruck/bromq/internal/storage"
)

// Engine manages script execution, state, and lifecycle
type Engine struct {
	db              *storage.DB
	mqttServer      *mqtt.Server
	state           *StateManager
	runtime         *Runtime
	logRetention    time.Duration // How long to keep logs (0 = forever)
	cleanupInterval time.Duration // How often to run cleanup
	cleanupTicker   *time.Ticker
	stopChan        chan struct{}
	wg              sync.WaitGroup
	shutdownMux     sync.Mutex
	isShutdown      bool
}

// NewEngine creates a new script engine
func NewEngine(db *storage.DB, mqttServer *mqtt.Server) *Engine {
	state := NewStateManager(db)
	runtime := NewRuntime(db, state, mqttServer)

	// Load log retention configuration
	logRetention := loadLogRetentionConfig()
	cleanupInterval := CalculateCleanupInterval(logRetention)

	if logRetention > 0 {
		slog.Info("Script log cleanup enabled",
			"retention", FormatDuration(logRetention),
			"check_interval", FormatDuration(cleanupInterval))
	} else {
		slog.Info("Script log cleanup disabled (logs kept forever)")
	}

	return &Engine{
		db:              db,
		mqttServer:      mqttServer,
		state:           state,
		runtime:         runtime,
		logRetention:    logRetention,
		cleanupInterval: cleanupInterval,
		stopChan:        make(chan struct{}),
	}
}

// loadLogRetentionConfig loads the log retention configuration from environment
func loadLogRetentionConfig() time.Duration {
	retentionStr := os.Getenv("SCRIPT_LOG_RETENTION")
	if retentionStr == "" {
		retentionStr = "30d" // Default: 30 days
	}

	retention, err := ParseDurationWithDays(retentionStr)
	if err != nil {
		slog.Warn("Invalid SCRIPT_LOG_RETENTION, using default",
			"value", retentionStr,
			"error", err,
			"default", "30d")
		return 30 * 24 * time.Hour
	}

	return retention
}

// Start starts the script engine and background workers
func (e *Engine) Start() {
	e.state.Start()

	// Start log cleanup worker if retention is configured
	if e.logRetention > 0 && e.cleanupInterval > 0 {
		e.wg.Add(1)
		go e.logCleanupWorker()
	}

	slog.Info("Script engine started")
}

// Shutdown gracefully shuts down the script engine
func (e *Engine) Shutdown(ctx context.Context) error {
	e.shutdownMux.Lock()
	if e.isShutdown {
		e.shutdownMux.Unlock()
		return nil
	}
	e.isShutdown = true
	e.shutdownMux.Unlock()

	slog.Info("Script engine shutdown initiated")

	// Stop accepting new executions
	close(e.stopChan)

	// Wait for in-flight scripts to complete (with timeout from context)
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All scripts completed gracefully")
	case <-ctx.Done():
		slog.Warn("Shutdown timeout reached, forcing stop")
	}

	// Stop state manager (includes final flush)
	if err := e.state.Stop(); err != nil {
		return fmt.Errorf("failed to stop state manager: %w", err)
	}

	slog.Info("Script engine shutdown complete")
	return nil
}

// ExecuteForTrigger executes all matching scripts for a given trigger and topic
func (e *Engine) ExecuteForTrigger(triggerType, topic string, event *Event) {
	// Check if shutting down
	select {
	case <-e.stopChan:
		slog.Debug("Script engine is shutting down, skipping execution")
		return
	default:
	}

	// Get matching scripts from database
	scripts, err := e.db.GetEnabledScriptsForTrigger(triggerType, topic)
	if err != nil {
		slog.Error("Failed to get scripts for trigger", "trigger", triggerType, "error", err)
		return
	}

	if len(scripts) == 0 {
		return // No scripts to execute
	}

	slog.Debug("Executing scripts for trigger",
		"trigger", triggerType,
		"topic", topic,
		"script_count", len(scripts))

	// Execute each script asynchronously
	for _, script := range scripts {
		e.wg.Add(1)
		go func(s storage.Script) {
			defer e.wg.Done()
			e.executeScript(&s, event)
		}(script)
	}
}

// executeScript executes a single script
func (e *Engine) executeScript(script *storage.Script, event *Event) {
	// Prevent self-triggering: if this script published the message, skip execution
	if event.PublishedByScriptID != nil && *event.PublishedByScriptID == script.ID {
		slog.Debug("Skipping self-triggered script",
			"script", script.Name,
			"trigger", event.Type,
			"topic", event.Topic)
		return
	}

	ctx := context.Background()

	slog.Debug("Executing script",
		"script", script.Name,
		"trigger", event.Type,
		"topic", event.Topic,
		"client", event.ClientID)

	result := e.runtime.Execute(ctx, script, event)

	if !result.Success {
		slog.Error("Script execution failed",
			"script", script.Name,
			"trigger", event.Type,
			"error", result.Error,
			"execution_time_ms", result.ExecutionTimeMs)
	} else {
		slog.Debug("Script executed successfully",
			"script", script.Name,
			"trigger", event.Type,
			"execution_time_ms", result.ExecutionTimeMs)
	}
}

// TestScript tests a script with mock event data (for API testing endpoint)
func (e *Engine) TestScript(scriptContent string, triggerType string, eventData map[string]interface{}) *ExecutionResult {
	// Create mock script
	script := &storage.Script{
		ID:            0, // Test script has no ID
		Name:          "test-script",
		ScriptContent: scriptContent,
		Enabled:       true,
	}

	// Build event from provided data
	event := &Event{
		Type: triggerType,
	}

	// Populate event fields from eventData
	if topic, ok := eventData["topic"].(string); ok {
		event.Topic = topic
	}
	if payload, ok := eventData["payload"].(string); ok {
		event.Payload = payload
	}
	if clientID, ok := eventData["clientId"].(string); ok {
		event.ClientID = clientID
	}
	if username, ok := eventData["username"].(string); ok {
		event.Username = username
	}
	if qos, ok := eventData["qos"].(float64); ok {
		event.QoS = byte(qos)
	}
	if retain, ok := eventData["retain"].(bool); ok {
		event.Retain = retain
	}

	// Execute script
	ctx := context.Background()
	return e.runtime.Execute(ctx, script, event)
}

// GetState returns the state manager (for API access)
func (e *Engine) GetState() *StateManager {
	return e.state
}

// GetDB returns the database (for API access)
func (e *Engine) GetDB() *storage.DB {
	return e.db
}

// logCleanupWorker periodically cleans up old script logs
func (e *Engine) logCleanupWorker() {
	defer e.wg.Done()

	e.cleanupTicker = time.NewTicker(e.cleanupInterval)
	defer e.cleanupTicker.Stop()

	// Don't run cleanup immediately - wait for first interval
	// This ensures database schema is fully ready in all environments

	for {
		select {
		case <-e.cleanupTicker.C:
			e.cleanupOldLogs()
		case <-e.stopChan:
			slog.Debug("Log cleanup worker stopping")
			return
		}
	}
}

// cleanupOldLogs deletes logs older than the retention period
func (e *Engine) cleanupOldLogs() {
	cutoff := time.Now().Add(-e.logRetention)

	slog.Debug("Running script log cleanup", "cutoff", cutoff.Format(time.RFC3339))

	if err := e.db.ClearAllScriptLogsBefore(cutoff); err != nil {
		slog.Error("Failed to cleanup old script logs", "error", err)
		return
	}

	slog.Debug("Script log cleanup completed")
}
