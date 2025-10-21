package mqtt

// Config holds MQTT server configuration
type Config struct {
	TCPAddr       string // TCP listener address (e.g., ":1883")
	WSAddr        string // WebSocket listener address (e.g., ":8883")
	EnableTLS     bool   // Enable TLS
	TLSCertFile   string // TLS certificate file path
	TLSKeyFile    string // TLS key file path
	MaxClients    int    // Maximum number of concurrent clients (0 = unlimited)
	RetainAvailable bool // Enable retained messages
}

// DefaultConfig returns a default MQTT configuration
func DefaultConfig() *Config {
	return &Config{
		TCPAddr:         ":1883",
		WSAddr:          ":8883",
		EnableTLS:       false,
		MaxClients:      0, // Unlimited
		RetainAvailable: true,
	}
}
