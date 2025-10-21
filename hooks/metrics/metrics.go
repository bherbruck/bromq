package metrics

import (
	"bytes"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// MetricsRecorder interface for recording client metrics
type MetricsRecorder interface {
	RegisterClient(clientID string)
	UnregisterClient(clientID string)
	RecordMessageReceived(clientID string, bytes int64)
	RecordMessageSent(clientID string, bytes int64)
	RecordPacketReceived(clientID string, bytes int64)
	RecordPacketSent(clientID string, bytes int64)
}

// MetricsHook implements MQTT hooks for metrics tracking
type MetricsHook struct {
	mqtt.HookBase
	recorder MetricsRecorder
}

// NewMetricsHook creates a new metrics hook
func NewMetricsHook(recorder MetricsRecorder) *MetricsHook {
	return &MetricsHook{
		recorder: recorder,
	}
}

// ID returns the hook identifier
func (h *MetricsHook) ID() string {
	return "metrics-tracker"
}

// Provides indicates which hook methods this hook provides
func (h *MetricsHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnPacketRead,
		mqtt.OnPacketSent,
	}, []byte{b})
}

// OnConnect is called when a client connects
func (h *MetricsHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	h.recorder.RegisterClient(cl.ID)
	return nil
}

// OnDisconnect is called when a client disconnects
func (h *MetricsHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.recorder.UnregisterClient(cl.ID)
}

// OnPacketRead is called when a packet is received from a client
func (h *MetricsHook) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// Estimate packet size (this is approximate)
	size := int64(pk.FixedHeader.Remaining + 2) // +2 for fixed header
	h.recorder.RecordPacketReceived(cl.ID, size)

	// Count PUBLISH packets as messages (type 3 = PUBLISH)
	if pk.FixedHeader.Type == 3 {
		h.recorder.RecordMessageReceived(cl.ID, size)
	}

	return pk, nil
}

// OnPacketSent is called when a packet is sent to a client
func (h *MetricsHook) OnPacketSent(cl *mqtt.Client, pk packets.Packet, b []byte) {
	size := int64(len(b))
	h.recorder.RecordPacketSent(cl.ID, size)

	// Count PUBLISH packets as messages (type 3 = PUBLISH)
	if pk.FixedHeader.Type == 3 {
		h.recorder.RecordMessageSent(cl.ID, size)
	}
}
