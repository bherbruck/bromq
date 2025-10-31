package script

import (
	"bytes"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	internalscript "github/bherbruck/bromq/internal/script"
)

// ScriptHook executes JavaScript scripts on MQTT events
type ScriptHook struct {
	mqtt.HookBase
	engine *internalscript.Engine
}

// NewScriptHook creates a new script hook
func NewScriptHook(engine *internalscript.Engine) *ScriptHook {
	return &ScriptHook{
		engine: engine,
	}
}

// ID returns the hook identifier
func (h *ScriptHook) ID() string {
	return "script-hook"
}

// Provides indicates which hook methods this hook provides
func (h *ScriptHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnPublish,
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnSubscribe,
		mqtt.OnSubscribed,
	}, []byte{b})
}

// OnPublish is called when a message is published
func (h *ScriptHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	event := &internalscript.Event{
		Type:     "publish",
		Topic:    pk.TopicName,
		Payload:  string(pk.Payload),
		ClientID: cl.ID,
		Username: string(cl.Properties.Username),
		QoS:      pk.FixedHeader.Qos,
		Retain:   pk.FixedHeader.Retain,
	}

	// Check if this message was published by a script (to prevent self-triggering)
	// Scripts use the inline client (ID: "inline")
	if cl.ID == "inline" {
		// Look up which script published this message
		event.PublishedByScriptID = internalscript.LookupScriptPublish(pk.TopicName, string(pk.Payload))
	}

	// Execute matching scripts asynchronously (don't block message flow)
	go h.engine.ExecuteForTrigger("on_publish", pk.TopicName, event)

	return pk, nil
}

// OnConnect is called when a client connects
func (h *ScriptHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	event := &internalscript.Event{
		Type:         "connect",
		ClientID:     cl.ID,
		Username:     string(cl.Properties.Username),
		CleanSession: pk.Connect.Clean,
	}

	// Execute matching scripts asynchronously
	go h.engine.ExecuteForTrigger("on_connect", "", event)

	return nil
}

// OnDisconnect is called when a client disconnects
func (h *ScriptHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	event := &internalscript.Event{
		Type:     "disconnect",
		ClientID: cl.ID,
		Username: string(cl.Properties.Username),
	}

	if err != nil {
		event.Error = err.Error()
	}

	// Execute matching scripts asynchronously
	go h.engine.ExecuteForTrigger("on_disconnect", "", event)
}

// OnSubscribe is called before a subscription is added
func (h *ScriptHook) OnSubscribe(cl *mqtt.Client, pk packets.Packet) packets.Packet {
	// OnSubscribe can have multiple filters
	for _, filter := range pk.Filters {
		event := &internalscript.Event{
			Type:     "subscribe",
			Topic:    filter.Filter,
			ClientID: cl.ID,
			Username: string(cl.Properties.Username),
			QoS:      filter.Qos,
		}

		// Execute matching scripts asynchronously
		go h.engine.ExecuteForTrigger("on_subscribe", filter.Filter, event)
	}

	return pk
}

// OnSubscribed is called after a subscription is added (alternative to OnSubscribe)
// Some users might prefer this over OnSubscribe
func (h *ScriptHook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	// This fires after subscription is confirmed
	// We already handled it in OnSubscribe, but keeping this for completeness
}
