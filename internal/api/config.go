package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
)

// Config holds API server configuration
type Config struct {
	JWTSecret []byte
}

// LoadConfig loads API configuration from environment variables
func LoadConfig() *Config {
	config := &Config{}

	// Load JWT secret from environment or generate one
	jwtSecretEnv := os.Getenv("JWT_SECRET")
	if jwtSecretEnv != "" {
		config.JWTSecret = []byte(jwtSecretEnv)
		slog.Info("JWT secret loaded from environment")
	} else {
		// Generate a secure random secret
		secret := make([]byte, 32) // 256 bits
		if _, err := rand.Read(secret); err != nil {
			slog.Error("Failed to generate JWT secret", "error", err)
			os.Exit(1)
		}
		config.JWTSecret = secret
		slog.Warn("JWT_SECRET not set, generated random secret. Set JWT_SECRET environment variable for production.",
			"generated_secret_hex", hex.EncodeToString(secret))
	}

	return config
}
