package appconfig

import (
	"github/bherbruck/bromq/internal/api"
	"github/bherbruck/bromq/internal/mqtt"
	"github/bherbruck/bromq/internal/storage"
)

// Config holds all application configuration
type Config struct {
	Version    bool   `flag:"version,v" desc:"Show version and exit"`
	ConfigFile string `env:"CONFIG_FILE" flag:"config,c" desc:"Path to YAML configuration file for provisioning"`

	Database storage.DatabaseConfig `desc:"Database connection settings"`
	MQTT     mqtt.Config            `desc:"MQTT broker settings"`
	API      api.Config             `desc:"HTTP API server settings"`
	Logging  LogConfig              `desc:"Logging settings"`
	Admin    AdminConfig            `desc:"Default admin credentials (only used on first run)"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `env:"LOG_LEVEL" flag:"log-level" default:"info" desc:"Log level (debug, info, warn, error)"`
	Format string `env:"LOG_FORMAT" flag:"log-format" default:"text" desc:"Log format (text, json)"`
}

// AdminConfig holds default admin credentials (only used on first database initialization)
type AdminConfig struct {
	Username string `env:"ADMIN_USERNAME" flag:"admin-username" default:"admin" desc:"Default admin username (only used on first run)"`
	Password string `env:"ADMIN_PASSWORD" flag:"admin-password" default:"admin" desc:"Default admin password (only used on first run)"`
}

// PostParse runs post-parsing logic for all sub-configs
func (c *Config) PostParse() error {
	// Apply database defaults
	if err := c.Database.PostParse(); err != nil {
		return err
	}

	// Apply API defaults (JWT secret generation)
	if err := c.API.PostParse(); err != nil {
		return err
	}

	return nil
}
