package mqtt

import (
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
func (s *Server) GetMetrics() Metrics {
	info := s.Info

	return Metrics{
		Uptime:            time.Since(time.Unix(info.Started, 0)),
		ConnectedClients:  len(s.Clients.GetAll()),
		TotalClients:      int(info.ClientsConnected),
		MessagesReceived:  info.MessagesReceived,
		MessagesSent:      info.MessagesSent,
		MessagesDropped:   info.MessagesDropped,
		PacketsReceived:   info.PacketsReceived,
		PacketsSent:       info.PacketsSent,
		BytesReceived:     info.BytesReceived,
		BytesSent:         info.BytesSent,
		SubscriptionsTotal: int(info.Subscriptions),
		RetainedMessages:  int(info.Retained),
	}
}
