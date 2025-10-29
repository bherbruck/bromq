package storage

import (
	"fmt"
	"os"
	"strconv"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Type     string // "sqlite", "postgres", "mysql"
	FilePath string // for sqlite
	Host     string // for postgres/mysql
	Port     int    // for postgres/mysql
	User     string // for postgres/mysql
	Password string // for postgres/mysql
	DBName   string // for postgres/mysql
	SSLMode  string // for postgres (disable, require, verify-ca, verify-full)
}

// DefaultSQLiteConfig returns default SQLite configuration
func DefaultSQLiteConfig(filePath string) *DatabaseConfig {
	return &DatabaseConfig{
		Type:     "sqlite",
		FilePath: filePath,
	}
}

// LoadConfigFromEnv loads database configuration from environment variables
func LoadConfigFromEnv() *DatabaseConfig {
	dbType := getEnv("DB_TYPE", "sqlite")

	config := &DatabaseConfig{
		Type:     dbType,
		FilePath: getEnv("DB_PATH", "bromq.db"),
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", getDefaultPort(dbType)),
		User:     getEnv("DB_USER", "mqtt"),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:   getEnv("DB_NAME", "mqtt"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	return config
}

// ConnectionString builds the appropriate connection string for the database type
func (c *DatabaseConfig) ConnectionString() (string, error) {
	switch c.Type {
	case "sqlite":
		return c.FilePath, nil

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

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDefaultPort returns the default port for a database type
func getDefaultPort(dbType string) int {
	switch dbType {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	default:
		return 0
	}
}
