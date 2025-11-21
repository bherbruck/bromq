package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
)

// Config holds API server configuration
type Config struct {
	HTTPAddr  string `env:"HTTP_ADDR" flag:"http" default:":8080" desc:"HTTP API server address"`
	JWTSecret string `env:"JWT_SECRET" flag:"jwt-secret" desc:"JWT secret for token signing (auto-generated if not set)"`
}

// PostParse applies post-parsing logic (JWT secret generation if not provided)
func (c *Config) PostParse() error {
	if c.JWTSecret == "" {
		// Generate a secure random secret
		secret := make([]byte, 32) // 256 bits
		if _, err := rand.Read(secret); err != nil {
			slog.Error("Failed to generate JWT secret", "error", err)
			os.Exit(1)
		}
		c.JWTSecret = hex.EncodeToString(secret)
		slog.Warn("JWT_SECRET not set, generated random secret. Set JWT_SECRET environment variable for production.",
			"generated_secret_hex", c.JWTSecret)
	} else {
		slog.Info("JWT secret loaded from configuration")
	}
	return nil
}

// JWTSecretBytes returns the JWT secret as bytes
func (c *Config) JWTSecretBytes() []byte {
	return []byte(c.JWTSecret)
}
