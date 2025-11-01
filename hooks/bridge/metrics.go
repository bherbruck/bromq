package bridge

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds Prometheus metric collectors for bridge connections
type Metrics struct {
	connectionStatus     *prometheus.GaugeVec
	connectionAttempts   *prometheus.CounterVec
	connectionFailures   *prometheus.CounterVec
	messagesForwarded    *prometheus.CounterVec
	messagesDropped      *prometheus.CounterVec
	reconnectAttempts    *prometheus.CounterVec
	currentBackoff       *prometheus.GaugeVec
	lastConnectedTime    *prometheus.GaugeVec
	lastDisconnectedTime *prometheus.GaugeVec
}

// NewMetrics creates a new bridge metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		connectionStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bridge_connection_status",
				Help: "Bridge connection status (1 = connected, 0 = disconnected)",
			},
			[]string{"bridge_name", "remote_host"},
		),
		connectionAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bridge_connection_attempts_total",
				Help: "Total number of bridge connection attempts",
			},
			[]string{"bridge_name", "remote_host"},
		),
		connectionFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bridge_connection_failures_total",
				Help: "Total number of bridge connection failures",
			},
			[]string{"bridge_name", "remote_host", "error_type"},
		),
		messagesForwarded: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bridge_messages_forwarded_total",
				Help: "Total number of messages forwarded through bridge",
			},
			[]string{"bridge_name", "direction"}, // direction: in, out
		),
		messagesDropped: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bridge_messages_dropped_total",
				Help: "Total number of messages dropped by bridge",
			},
			[]string{"bridge_name", "direction", "reason"},
		),
		reconnectAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bridge_reconnect_attempts_total",
				Help: "Total number of bridge reconnection attempts",
			},
			[]string{"bridge_name"},
		),
		currentBackoff: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bridge_current_backoff_seconds",
				Help: "Current exponential backoff delay in seconds",
			},
			[]string{"bridge_name"},
		),
		lastConnectedTime: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bridge_last_connected_timestamp_seconds",
				Help: "Unix timestamp when bridge last connected",
			},
			[]string{"bridge_name"},
		),
		lastDisconnectedTime: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bridge_last_disconnected_timestamp_seconds",
				Help: "Unix timestamp when bridge last disconnected",
			},
			[]string{"bridge_name"},
		),
	}
}

// SetConnectionStatus sets the connection status for a bridge
func (m *Metrics) SetConnectionStatus(bridgeName, remoteHost string, connected bool) {
	var status float64
	if connected {
		status = 1
		m.lastConnectedTime.WithLabelValues(bridgeName).SetToCurrentTime()
	} else {
		status = 0
		m.lastDisconnectedTime.WithLabelValues(bridgeName).SetToCurrentTime()
	}
	m.connectionStatus.WithLabelValues(bridgeName, remoteHost).Set(status)
}

// RecordConnectionAttempt records a connection attempt
func (m *Metrics) RecordConnectionAttempt(bridgeName, remoteHost string) {
	m.connectionAttempts.WithLabelValues(bridgeName, remoteHost).Inc()
}

// RecordConnectionFailure records a connection failure
func (m *Metrics) RecordConnectionFailure(bridgeName, remoteHost, errorType string) {
	m.connectionFailures.WithLabelValues(bridgeName, remoteHost, errorType).Inc()
}

// RecordMessageForwarded records a forwarded message
func (m *Metrics) RecordMessageForwarded(bridgeName, direction string) {
	m.messagesForwarded.WithLabelValues(bridgeName, direction).Inc()
}

// RecordMessageDropped records a dropped message
func (m *Metrics) RecordMessageDropped(bridgeName, direction, reason string) {
	m.messagesDropped.WithLabelValues(bridgeName, direction, reason).Inc()
}

// RecordReconnectAttempt records a reconnection attempt
func (m *Metrics) RecordReconnectAttempt(bridgeName string) {
	m.reconnectAttempts.WithLabelValues(bridgeName).Inc()
}

// SetCurrentBackoff sets the current backoff delay
func (m *Metrics) SetCurrentBackoff(bridgeName string, backoffSeconds float64) {
	m.currentBackoff.WithLabelValues(bridgeName).Set(backoffSeconds)
}
