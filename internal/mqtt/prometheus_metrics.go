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
