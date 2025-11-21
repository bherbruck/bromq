package mqtt

// Config holds MQTT server configuration
type Config struct {
	TCPAddr         string `env:"MQTT_TCP_ADDR" flag:"mqtt-tcp" default:":1883" desc:"MQTT TCP listener address"`
	WSAddr          string `env:"MQTT_WS_ADDR" flag:"mqtt-ws" default:":8883" desc:"MQTT WebSocket listener address"`
	EnableTLS       bool   `env:"MQTT_ENABLE_TLS" flag:"mqtt-tls" desc:"Enable TLS for MQTT connections"`
	TLSCertFile     string `env:"MQTT_TLS_CERT" flag:"mqtt-tls-cert" desc:"TLS certificate file path"`
	TLSKeyFile      string `env:"MQTT_TLS_KEY" flag:"mqtt-tls-key" desc:"TLS key file path"`
	MaxClients      int    `env:"MQTT_MAX_CLIENTS" flag:"mqtt-max-clients" default:"0" desc:"Maximum number of concurrent clients (0 = unlimited)"`
	RetainAvailable bool   `env:"MQTT_RETAIN_AVAILABLE" flag:"mqtt-retain" default:"true" desc:"Enable retained messages"`
	AllowAnonymous  bool   `env:"MQTT_ALLOW_ANONYMOUS" flag:"mqtt-allow-anonymous" desc:"Allow clients to connect without credentials (insecure)"`
}

// DefaultConfig returns a default MQTT configuration
func DefaultConfig() *Config {
	return &Config{
		TCPAddr:         ":1883",
		WSAddr:          ":8883",
		EnableTLS:       false,
		MaxClients:      0, // Unlimited
		RetainAvailable: true,
		AllowAnonymous:  false, // Disabled by default for security
	}
}
