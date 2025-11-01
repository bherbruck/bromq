package mqtt

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics holds Prometheus metric collectors for MQTT
type PrometheusMetrics struct {
	messagesReceived    *prometheus.CounterVec
	messagesSent        *prometheus.CounterVec
	bytesReceived       *prometheus.CounterVec
	bytesSent           *prometheus.CounterVec
	packetsReceived     *prometheus.CounterVec
	packetsSent         *prometheus.CounterVec
	clientsConnected    prometheus.Gauge
	clientConnectedTime *prometheus.GaugeVec
	// ACL metrics
	aclChecks    *prometheus.CounterVec
	aclDenied    *prometheus.CounterVec
	authAttempts *prometheus.CounterVec
	authFailures *prometheus.CounterVec
}

// NewPrometheusMetrics creates a new Prometheus metrics collector
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		messagesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_messages_received_total",
				Help: "Total number of PUBLISH messages received from clients",
			},
			[]string{"client_id"},
		),
		messagesSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_messages_sent_total",
				Help: "Total number of PUBLISH messages sent to clients",
			},
			[]string{"client_id"},
		),
		bytesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_bytes_received_total",
				Help: "Total bytes received from clients",
			},
			[]string{"client_id"},
		),
		bytesSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_bytes_sent_total",
				Help: "Total bytes sent to clients",
			},
			[]string{"client_id"},
		),
		packetsReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_packets_received_total",
				Help: "Total MQTT packets received from clients",
			},
			[]string{"client_id"},
		),
		packetsSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_packets_sent_total",
				Help: "Total MQTT packets sent to clients",
			},
			[]string{"client_id"},
		),
		clientsConnected: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "mqtt_clients_connected",
				Help: "Number of currently connected MQTT clients",
			},
		),
		clientConnectedTime: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mqtt_client_connected_timestamp_seconds",
				Help: "Unix timestamp when client connected",
			},
			[]string{"client_id"},
		),
		aclChecks: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_acl_checks_total",
				Help: "Total number of ACL authorization checks",
			},
			[]string{"username", "action", "result"},
		),
		aclDenied: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_acl_denied_total",
				Help: "Total number of ACL denials (security monitoring)",
			},
			[]string{"username", "action", "topic"},
		),
		authAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_auth_attempts_total",
				Help: "Total number of authentication attempts",
			},
			[]string{"username", "result"},
		),
		authFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mqtt_auth_failures_total",
				Help: "Total number of authentication failures (security monitoring)",
			},
			[]string{"username"},
		),
	}
}

// RegisterClient increments the connected clients gauge
func (pm *PrometheusMetrics) RegisterClient(clientID string) {
	pm.clientsConnected.Inc()
	pm.clientConnectedTime.WithLabelValues(clientID).SetToCurrentTime()
}

// UnregisterClient decrements the connected clients gauge
func (pm *PrometheusMetrics) UnregisterClient(clientID string) {
	pm.clientsConnected.Dec()
	pm.clientConnectedTime.DeleteLabelValues(clientID)
}

// RecordMessageReceived records a received message
func (pm *PrometheusMetrics) RecordMessageReceived(clientID string, bytes int64) {
	pm.messagesReceived.WithLabelValues(clientID).Inc()
}

// RecordMessageSent records a sent message
func (pm *PrometheusMetrics) RecordMessageSent(clientID string, bytes int64) {
	pm.messagesSent.WithLabelValues(clientID).Inc()
}

// RecordPacketReceived records a received packet
func (pm *PrometheusMetrics) RecordPacketReceived(clientID string, bytes int64) {
	pm.packetsReceived.WithLabelValues(clientID).Inc()
	pm.bytesReceived.WithLabelValues(clientID).Add(float64(bytes))
}

// RecordPacketSent records a sent packet
func (pm *PrometheusMetrics) RecordPacketSent(clientID string, bytes int64) {
	pm.packetsSent.WithLabelValues(clientID).Inc()
	pm.bytesSent.WithLabelValues(clientID).Add(float64(bytes))
}

// RecordACLCheck records an ACL authorization check
func (pm *PrometheusMetrics) RecordACLCheck(username, action, result string) {
	pm.aclChecks.WithLabelValues(username, action, result).Inc()
}

// RecordACLDenied records an ACL denial (for security monitoring)
func (pm *PrometheusMetrics) RecordACLDenied(username, action, topic string) {
	pm.aclDenied.WithLabelValues(username, action, topic).Inc()
}

// RecordAuthAttempt records an authentication attempt
func (pm *PrometheusMetrics) RecordAuthAttempt(username, result string) {
	pm.authAttempts.WithLabelValues(username, result).Inc()
}

// RecordAuthFailure records an authentication failure (for security monitoring)
func (pm *PrometheusMetrics) RecordAuthFailure(username string) {
	pm.authFailures.WithLabelValues(username).Inc()
}
