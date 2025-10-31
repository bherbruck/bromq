package script

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dop251/goja"
	mqtt "github.com/mochi-mqtt/server/v2"

	"github/bherbruck/bromq/internal/storage"
)

// ExecutionResult contains the result of script execution
type ExecutionResult struct {
	Success         bool
	Error           error
	Logs            []ScriptLogEntry
	ExecutionTimeMs int
}

// Runtime handles individual script execution with timeout and error handling
type Runtime struct {
	db         *storage.DB
	state      *StateManager
	mqttServer *mqtt.Server
	timeout    time.Duration
}

// NewRuntime creates a new runtime
func NewRuntime(db *storage.DB, state *StateManager, mqttServer *mqtt.Server) *Runtime {
	return &Runtime{
		db:         db,
		state:      state,
		mqttServer: mqttServer,
		timeout:    1 * time.Second, // Default 1 second timeout
	}
}

// SetTimeout sets the execution timeout
func (r *Runtime) SetTimeout(timeout time.Duration) {
	r.timeout = timeout
}

// Execute runs a script with the given event context
func (r *Runtime) Execute(ctx context.Context, script *storage.Script, event *Event) *ExecutionResult {
	startTime := time.Now()

	result := &ExecutionResult{
		Success: false,
		Logs:    make([]ScriptLogEntry, 0),
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Execute in goroutine to handle timeout
	done := make(chan bool)
	var execErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("script panic: %v", r)
				slog.Error("Script panic",
					"script", script.Name,
					"error", execErr,
					"trigger", event.Type)
			}
			done <- true
		}()

		// Create new Goja VM for this execution
		vm := goja.New()

		// Set up APIs
		api := NewScriptAPI(vm, script.ID, script.Name, event.Type, r.state, r.mqttServer)

		// Convert Event to map with JSON field names for JavaScript access
		eventMap := map[string]interface{}{
			"type":         event.Type,
			"topic":        event.Topic,
			"payload":      event.Payload,
			"clientId":     event.ClientID,
			"username":     event.Username,
			"qos":          event.QoS,
			"retain":       event.Retain,
			"cleanSession": event.CleanSession,
			"error":        event.Error,
		}

		// Set event object in scope
		vm.Set("event", eventMap)

		// Compile and run script
		program, err := goja.Compile(script.Name, script.ScriptContent, false)
		if err != nil {
			execErr = fmt.Errorf("compilation error: %w", err)
			return
		}

		_, err = vm.RunProgram(program)
		if err != nil {
			execErr = fmt.Errorf("runtime error: %w", err)
			return
		}

		// Collect logs
		result.Logs = api.GetLogs()
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Execution completed
		result.ExecutionTimeMs = int(time.Since(startTime).Milliseconds())

		if execErr != nil {
			result.Error = execErr
			result.Success = false
		} else {
			result.Success = true
		}

	case <-execCtx.Done():
		// Timeout
		result.ExecutionTimeMs = int(time.Since(startTime).Milliseconds())
		result.Error = fmt.Errorf("execution timeout after %v", r.timeout)
		result.Success = false

		slog.Warn("Script execution timeout",
			"script", script.Name,
			"trigger", event.Type,
			"timeout", r.timeout)
	}

	// Log execution to database
	r.logExecution(script.ID, event, result)

	return result
}

// logExecution logs the script execution to the database
func (r *Runtime) logExecution(scriptID uint, event *Event, result *ExecutionResult) {
	// Create context with event details
	context := event.ToJSON()

	// Only auto-log errors/failures (reduces noise for high-frequency scripts)
	if !result.Success {
		level := "error"
		message := "Script execution failed"
		if result.Error != nil {
			message = result.Error.Error()
		}

		if err := r.db.CreateScriptLog(
			scriptID,
			event.Type,
			level,
			message,
			context,
			result.ExecutionTimeMs,
		); err != nil {
			slog.Error("Failed to create script log", "error", err)
		}
	}

	// Always log user messages from the script (log.info, log.warn, etc.)
	for _, logEntry := range result.Logs {
		if err := r.db.CreateScriptLog(
			scriptID,
			event.Type,
			logEntry.Level,
			logEntry.Message,
			context,
			0, // User logs don't have execution time
		); err != nil {
			slog.Error("Failed to create script log", "error", err)
		}
	}
}
