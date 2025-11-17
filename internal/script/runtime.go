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
	db             *storage.DB
	state          *StateManager
	mqttServer     *mqtt.Server
	defaultTimeout time.Duration
	maxPublishes   int
}

// NewRuntime creates a new runtime
func NewRuntime(db *storage.DB, state *StateManager, mqttServer *mqtt.Server) *Runtime {
	return &Runtime{
		db:             db,
		state:          state,
		mqttServer:     mqttServer,
		defaultTimeout: 5 * time.Second, // Default 5 seconds timeout (will be overridden by engine)
		maxPublishes:   100,              // Default 100 publishes per execution (will be overridden by engine)
	}
}

// SetDefaultTimeout sets the default execution timeout
func (r *Runtime) SetDefaultTimeout(timeout time.Duration) {
	r.defaultTimeout = timeout
}

// SetMaxPublishes sets the max publishes per execution limit
func (r *Runtime) SetMaxPublishes(maxPublishes int) {
	r.maxPublishes = maxPublishes
}

// Execute runs a script with the given message context
func (r *Runtime) Execute(ctx context.Context, script *storage.Script, message *Message) *ExecutionResult {
	startTime := time.Now()

	result := &ExecutionResult{
		Success: false,
		Logs:    make([]ScriptLogEntry, 0),
	}

	// Determine timeout to use: script-specific or default
	timeout := r.defaultTimeout
	if script.TimeoutSeconds != nil && *script.TimeoutSeconds > 0 {
		timeout = time.Duration(*script.TimeoutSeconds) * time.Second
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute in goroutine to handle timeout
	done := make(chan bool)
	var execErr error
	var vm *goja.Runtime // Store VM reference for interrupt

	go func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("script panic: %v", r)
				slog.Error("Script panic",
					"script", script.Name,
					"error", execErr,
					"trigger", message.Type)
			}
			done <- true
		}()

		// Create new Goja VM for this execution
		vm = goja.New()

		// Set up APIs
		api := NewScriptAPI(vm, script.ID, script.Name, message.Type, r.state, r.mqttServer, r.maxPublishes)

		// Convert Message to map with JSON field names for JavaScript access
		msgMap := map[string]interface{}{
			"type":         message.Type,
			"topic":        message.Topic,
			"payload":      message.Payload,
			"clientId":     message.ClientID,
			"username":     message.Username,
			"qos":          message.QoS,
			"retain":       message.Retain,
			"cleanSession": message.CleanSession,
			"error":        message.Error,
		}

		// Set msg object in scope
		_ = vm.Set("msg", msgMap)

		// Compile and run script
		program, err := goja.Compile(script.Name, script.Content, false)
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
		// Timeout - interrupt the VM to stop execution
		if vm != nil {
			vm.Interrupt("execution timeout")
		}

		// Wait for goroutine to finish after interrupt (with a safety timeout)
		select {
		case <-done:
			// Goroutine finished after interrupt
		case <-time.After(100 * time.Millisecond):
			// Safety timeout: goroutine didn't finish, log warning but continue
			slog.Error("Script goroutine did not terminate after interrupt",
				"script", script.Name,
				"trigger", message.Type)
		}

		result.ExecutionTimeMs = int(time.Since(startTime).Milliseconds())
		result.Error = fmt.Errorf("execution timeout after %v", timeout)
		result.Success = false

		slog.Warn("Script execution timeout",
			"script", script.Name,
			"trigger", message.Type,
			"timeout", timeout)
	}

	// Log execution to database
	r.logExecution(script.ID, message, result)

	return result
}

// logExecution logs the script execution to the database
func (r *Runtime) logExecution(scriptID uint, message *Message, result *ExecutionResult) {
	// Create context with message details
	context := message.ToJSON()

	// Only auto-log errors/failures (reduces noise for high-frequency scripts)
	if !result.Success {
		level := "error"
		msg := "Script execution failed"
		if result.Error != nil {
			msg = result.Error.Error()
		}

		if err := r.db.CreateScriptLog(
			scriptID,
			message.Type,
			level,
			msg,
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
			message.Type,
			logEntry.Level,
			logEntry.Message,
			context,
			0, // User logs don't have execution time
		); err != nil {
			slog.Error("Failed to create script log", "error", err)
		}
	}
}
