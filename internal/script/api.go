package script

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dop251/goja"
	mqtt "github.com/mochi-mqtt/server/v2"
	"gorm.io/datatypes"
)

// Global tracking of script-published messages to prevent self-triggering
var (
	scriptPublishTracker = &publishTracker{
		publishes: make(map[string]*publishRecord),
	}
)

type publishRecord struct {
	scriptID  uint
	expiresAt time.Time
}

type publishTracker struct {
	mu        sync.RWMutex
	publishes map[string]*publishRecord // key: hash of topic+payload
}

func (pt *publishTracker) track(topic, payload string, scriptID uint) {
	key := pt.makeKey(topic, payload)
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.publishes[key] = &publishRecord{
		scriptID:  scriptID,
		expiresAt: time.Now().Add(100 * time.Millisecond), // Very short TTL
	}
}

func (pt *publishTracker) lookup(topic, payload string) *uint {
	key := pt.makeKey(topic, payload)
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	record, exists := pt.publishes[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(record.expiresAt) {
		return nil
	}

	return &record.scriptID
}

func (pt *publishTracker) cleanup() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	for key, record := range pt.publishes {
		if now.After(record.expiresAt) {
			delete(pt.publishes, key)
		}
	}
}

func (pt *publishTracker) makeKey(topic, payload string) string {
	// Hash topic+payload to create a unique key
	h := sha256.New()
	h.Write([]byte(topic))
	h.Write([]byte("|"))
	h.Write([]byte(payload))
	return string(h.Sum(nil))
}

// LookupScriptPublish checks if a message was recently published by a script
// Returns the script ID if found, nil otherwise
func LookupScriptPublish(topic, payload string) *uint {
	return scriptPublishTracker.lookup(topic, payload)
}

// CleanupScriptPublishTracker removes expired tracking entries
func CleanupScriptPublishTracker() {
	scriptPublishTracker.cleanup()
}

// ScriptAPI provides JavaScript APIs for scripts
type ScriptAPI struct {
	vm           *goja.Runtime
	scriptID     uint
	scriptName   string
	triggerType  string
	state        StateStore
	mqttServer   *mqtt.Server
	logs         []ScriptLogEntry
	publishCount int // Track publishes in this execution
	maxPublishes int // Rate limit: max publishes per execution
}

// ScriptLogEntry represents a log entry from a script
type ScriptLogEntry struct {
	Level   string
	Message string
}

// NewScriptAPI creates a new script API instance
func NewScriptAPI(vm *goja.Runtime, scriptID uint, scriptName, triggerType string, state StateStore, mqttServer *mqtt.Server, maxPublishes int) *ScriptAPI {
	api := &ScriptAPI{
		vm:           vm,
		scriptID:     scriptID,
		scriptName:   scriptName,
		triggerType:  triggerType,
		state:        state,
		mqttServer:   mqttServer,
		logs:         make([]ScriptLogEntry, 0),
		publishCount: 0,
		maxPublishes: maxPublishes,
	}

	api.setupAPIs()
	return api
}

// setupAPIs registers all JavaScript APIs
func (api *ScriptAPI) setupAPIs() {
	// Create log object
	logObj := api.vm.NewObject()
	_ = logObj.Set("debug", api.logDebug)
	_ = logObj.Set("info", api.logInfo)
	_ = logObj.Set("warn", api.logWarn)
	_ = logObj.Set("error", api.logError)
	_ = api.vm.Set("log", logObj)

	// Create mqtt object
	mqttObj := api.vm.NewObject()
	_ = mqttObj.Set("publish", api.mqttPublish)
	_ = api.vm.Set("mqtt", mqttObj)

	// Create state object (script-scoped)
	stateObj := api.vm.NewObject()
	_ = stateObj.Set("set", api.stateSet)
	_ = stateObj.Set("get", api.stateGet)
	_ = stateObj.Set("delete", api.stateDelete)
	_ = stateObj.Set("keys", api.stateKeys)
	_ = api.vm.Set("state", stateObj)

	// Create global object (shared across all scripts)
	globalObj := api.vm.NewObject()
	_ = globalObj.Set("set", api.globalSet)
	_ = globalObj.Set("get", api.globalGet)
	_ = globalObj.Set("delete", api.globalDelete)
	_ = globalObj.Set("keys", api.globalKeys)
	_ = api.vm.Set("global", globalObj)
}

// GetLogs returns all collected logs
func (api *ScriptAPI) GetLogs() []ScriptLogEntry {
	return api.logs
}

// Log functions

func (api *ScriptAPI) logDebug(call goja.FunctionCall) goja.Value {
	msg := api.formatLogMessage(call.Arguments)
	api.logs = append(api.logs, ScriptLogEntry{Level: "debug", Message: msg})
	slog.Debug(msg, "script", api.scriptName, "trigger", api.triggerType)
	return goja.Undefined()
}

func (api *ScriptAPI) logInfo(call goja.FunctionCall) goja.Value {
	msg := api.formatLogMessage(call.Arguments)
	api.logs = append(api.logs, ScriptLogEntry{Level: "info", Message: msg})
	slog.Info(msg, "script", api.scriptName, "trigger", api.triggerType)
	return goja.Undefined()
}

func (api *ScriptAPI) logWarn(call goja.FunctionCall) goja.Value {
	msg := api.formatLogMessage(call.Arguments)
	api.logs = append(api.logs, ScriptLogEntry{Level: "warn", Message: msg})
	slog.Warn(msg, "script", api.scriptName, "trigger", api.triggerType)
	return goja.Undefined()
}

func (api *ScriptAPI) logError(call goja.FunctionCall) goja.Value {
	msg := api.formatLogMessage(call.Arguments)
	api.logs = append(api.logs, ScriptLogEntry{Level: "error", Message: msg})
	slog.Error(msg, "script", api.scriptName, "trigger", api.triggerType)
	return goja.Undefined()
}

func (api *ScriptAPI) formatLogMessage(args []goja.Value) string {
	if len(args) == 0 {
		return ""
	}

	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = fmt.Sprint(arg.Export())
	}

	// Join with spaces like console.log
	msg := ""
	for i, part := range parts {
		if i > 0 {
			msg += " "
		}
		msg += part
	}

	return msg
}

// MQTT functions

func (api *ScriptAPI) mqttPublish(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(api.vm.NewTypeError("mqtt.publish requires at least 2 arguments (topic, payload)"))
	}

	topic := call.Argument(0).String()
	payload := call.Argument(1).String()
	qos := byte(0)
	retain := false

	if len(call.Arguments) >= 3 {
		qos = byte(call.Argument(2).ToInteger())
	}
	if len(call.Arguments) >= 4 {
		retain = call.Argument(3).ToBoolean()
	}

	// Validate QoS
	if qos > 2 {
		panic(api.vm.NewTypeError("QoS must be 0, 1, or 2"))
	}

	// Check publish rate limit (prevent infinite loop spam)
	if api.publishCount >= api.maxPublishes {
		panic(api.vm.NewTypeError(fmt.Sprintf("publish rate limit exceeded (max %d per execution)", api.maxPublishes)))
	}
	api.publishCount++

	// Track this publish to prevent self-triggering (expires in 100ms)
	scriptPublishTracker.track(topic, payload, api.scriptID)

	// Publish to MQTT server
	if err := api.mqttServer.Publish(topic, []byte(payload), retain, qos); err != nil {
		slog.Error("Failed to publish from script", "script", api.scriptName, "topic", topic, "error", err)
		panic(api.vm.NewGoError(fmt.Errorf("failed to publish: %w", err)))
	}

	return goja.Undefined()
}

// State functions (script-scoped)

func (api *ScriptAPI) stateSet(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(api.vm.NewTypeError("state.set requires at least 2 arguments (key, value)"))
	}

	key := call.Argument(0).String()
	value := call.Argument(1).Export()

	var ttl *int
	if len(call.Arguments) >= 3 {
		opts := call.Argument(2).ToObject(api.vm)
		if opts != nil {
			if ttlVal := opts.Get("ttl"); ttlVal != nil && ttlVal != goja.Undefined() {
				ttlInt := int(ttlVal.ToInteger())
				ttl = &ttlInt
			}
		}
	}

	if err := api.state.Set(&api.scriptID, key, value, ttl); err != nil {
		panic(api.vm.NewGoError(err))
	}

	return goja.Undefined()
}

func (api *ScriptAPI) stateGet(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(api.vm.NewTypeError("state.get requires 1 argument (key)"))
	}

	key := call.Argument(0).String()
	value, ok := api.state.Get(&api.scriptID, key)

	if !ok {
		return goja.Undefined()
	}

	return api.vm.ToValue(value)
}

func (api *ScriptAPI) stateDelete(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(api.vm.NewTypeError("state.delete requires 1 argument (key)"))
	}

	key := call.Argument(0).String()
	if err := api.state.Delete(&api.scriptID, key); err != nil {
		panic(api.vm.NewGoError(err))
	}

	return goja.Undefined()
}

func (api *ScriptAPI) stateKeys(call goja.FunctionCall) goja.Value {
	keys := api.state.Keys(&api.scriptID)
	return api.vm.ToValue(keys)
}

// Global state functions (shared across all scripts)

func (api *ScriptAPI) globalSet(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(api.vm.NewTypeError("global.set requires at least 2 arguments (key, value)"))
	}

	key := call.Argument(0).String()
	value := call.Argument(1).Export()

	var ttl *int
	if len(call.Arguments) >= 3 {
		opts := call.Argument(2).ToObject(api.vm)
		if opts != nil {
			if ttlVal := opts.Get("ttl"); ttlVal != nil && ttlVal != goja.Undefined() {
				ttlInt := int(ttlVal.ToInteger())
				ttl = &ttlInt
			}
		}
	}

	if err := api.state.Set(nil, key, value, ttl); err != nil {
		panic(api.vm.NewGoError(err))
	}

	return goja.Undefined()
}

func (api *ScriptAPI) globalGet(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(api.vm.NewTypeError("global.get requires 1 argument (key)"))
	}

	key := call.Argument(0).String()
	value, ok := api.state.Get(nil, key)

	if !ok {
		return goja.Undefined()
	}

	return api.vm.ToValue(value)
}

func (api *ScriptAPI) globalDelete(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(api.vm.NewTypeError("global.delete requires 1 argument (key)"))
	}

	key := call.Argument(0).String()
	if err := api.state.Delete(nil, key); err != nil {
		panic(api.vm.NewGoError(err))
	}

	return goja.Undefined()
}

func (api *ScriptAPI) globalKeys(call goja.FunctionCall) goja.Value {
	keys := api.state.Keys(nil)
	return api.vm.ToValue(keys)
}

// Message represents the context passed to scripts
type Message struct {
	Type                string `json:"type"`
	Topic               string `json:"topic,omitempty"`
	Payload             string `json:"payload,omitempty"`
	ClientID            string `json:"clientId"`
	Username            string `json:"username"`
	QoS                 byte   `json:"qos,omitempty"`
	Retain              bool   `json:"retain,omitempty"`
	CleanSession        bool   `json:"cleanSession,omitempty"`
	Error               string `json:"error,omitempty"`
	PublishedByScriptID *uint  `json:"-"` // Internal: tracks which script published this message (prevents self-triggering)
}

// ToJSON converts message to JSON for logging
func (m *Message) ToJSON() datatypes.JSON {
	data, _ := json.Marshal(m)
	return datatypes.JSON(data)
}

// ToMap converts the message to a map for storage
func (m *Message) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["type"] = m.Type
	result["topic"] = m.Topic
	result["payload"] = string(m.Payload)
	result["client_id"] = m.ClientID
	result["username"] = m.Username
	result["qos"] = m.QoS
	result["retain"] = m.Retain
	return result
}
