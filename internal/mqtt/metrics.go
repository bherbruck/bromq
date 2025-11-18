package mqtt

import (
	"sync/atomic"
	"time"
)

// Metrics holds MQTT server metrics
type Metrics struct {
	Uptime            time.Duration `json:"uptime"`
	ConnectedClients  int           `json:"connected_clients"`
	TotalClients      int           `json:"total_clients"`
	MessagesReceived  int64         `json:"messages_received"`
	MessagesSent      int64         `json:"messages_sent"`
	MessagesDropped   int64         `json:"messages_dropped"`
	PacketsReceived   int64         `json:"packets_received"`
	PacketsSent       int64         `json:"packets_sent"`
	BytesReceived     int64         `json:"bytes_received"`
	BytesSent         int64         `json:"bytes_sent"`
	SubscriptionsTotal int          `json:"subscriptions_total"`
	RetainedMessages  int          `json:"retained_messages"`
}

// GetMetrics returns current server metrics
// Uses atomic loads to safely read counters that are updated concurrently
func (s *Server) GetMetrics() Metrics {
	info := s.Info

	return Metrics{
		Uptime:            time.Since(time.Unix(atomic.LoadInt64(&info.Started), 0)),
		ConnectedClients:  len(s.Clients.GetAll()),
		TotalClients:      int(atomic.LoadInt64(&info.ClientsConnected)),
		MessagesReceived:  atomic.LoadInt64(&info.MessagesReceived),
		MessagesSent:      atomic.LoadInt64(&info.MessagesSent),
		MessagesDropped:   atomic.LoadInt64(&info.MessagesDropped),
		PacketsReceived:   atomic.LoadInt64(&info.PacketsReceived),
		PacketsSent:       atomic.LoadInt64(&info.PacketsSent),
		BytesReceived:     atomic.LoadInt64(&info.BytesReceived),
		BytesSent:         atomic.LoadInt64(&info.BytesSent),
		SubscriptionsTotal: int(atomic.LoadInt64(&info.Subscriptions)),
		RetainedMessages:  int(atomic.LoadInt64(&info.Retained)),
	}
}
