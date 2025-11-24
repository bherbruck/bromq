package storage

import (
	"fmt"
	"strings"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Type     string `env:"DB_TYPE" flag:"db-type" default:"sqlite" desc:"Database type (sqlite, postgres, mysql)"`
	FilePath string `env:"DB_PATH" flag:"db-path" default:"bromq.db" desc:"SQLite database file path"`
	Host     string `env:"DB_HOST" flag:"db-host" default:"localhost" desc:"Database host (postgres/mysql)"`
	Port     int    `env:"DB_PORT" flag:"db-port" desc:"Database port (postgres/mysql). Auto-detected if not set"`
	User     string `env:"DB_USER" flag:"db-user" default:"mqtt" desc:"Database user (postgres/mysql)"`
	Password string `env:"DB_PASSWORD" flag:"db-password" desc:"Database password (postgres/mysql)"`
	DBName   string `env:"DB_NAME" flag:"db-name" default:"mqtt" desc:"Database name (postgres/mysql)"`
	SSLMode  string `env:"DB_SSLMODE" flag:"db-sslmode" default:"disable" desc:"SSL mode for postgres (disable, require, verify-ca, verify-full)"`
}

// DefaultSQLiteConfig returns default SQLite configuration
func DefaultSQLiteConfig(filePath string) *DatabaseConfig {
	return &DatabaseConfig{
		Type:     "sqlite",
		FilePath: filePath,
	}
}


// PostParse applies defaults and validation after parsing
func (c *DatabaseConfig) PostParse() error {
	// Set default ports based on database type if not specified
	if c.Port == 0 {
		switch c.Type {
		case "postgres":
			c.Port = 5432
		case "mysql":
			c.Port = 3306
		}
	}
	return nil
}

// ConnectionString builds the appropriate connection string for the database type
func (c *DatabaseConfig) ConnectionString() (string, error) {
	switch c.Type {
	case "sqlite":
		// For in-memory databases (tests), no pragmas needed
		if c.FilePath == ":memory:" || strings.HasPrefix(c.FilePath, "file::memory:") {
			return c.FilePath, nil
		}
		// For file-based SQLite: Only enable foreign keys
		// With MaxOpenConns=1:
		// - WAL mode unnecessary (no concurrent readers)
		// - busy_timeout unnecessary (no lock contention)
		// - Simple DELETE mode = one file, easy backups
		return c.FilePath + "?_pragma=foreign_keys(1)", nil

	case "postgres":
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
			c.Host, c.User, c.Password, c.DBName, c.Port, c.SSLMode), nil

	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, c.Port, c.DBName), nil

	default:
		return "", fmt.Errorf("unsupported database type: %s (supported: sqlite, postgres, mysql)", c.Type)
	}
}
