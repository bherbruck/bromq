package storage

import (
	"fmt"
	"log/slog"

	sqlite "github.com/glebarez/sqlite" // Pure Go SQLite driver (no CGO required)
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB wraps the GORM database connection with in-memory caching
type DB struct {
	*gorm.DB
	cache *Cache
}

// Open creates a new database connection and runs auto-migrations
// Supports SQLite, PostgreSQL, and MySQL based on the provided configuration
func Open(config *DatabaseConfig) (*DB, error) {
	return OpenWithCache(config, nil)
}

// OpenWithCache creates a new database connection with a custom cache instance (for testing)
// If cache is nil, creates a new cache with the default Prometheus registry
func OpenWithCache(config *DatabaseConfig, cache *Cache) (*DB, error) {
	if config == nil {
		// Default to SQLite for backward compatibility
		config = DefaultSQLiteConfig("bromq.db")
	}

	// Get connection string
	dsn, err := config.ConnectionString()
	if err != nil {
		return nil, err
	}

	// Select appropriate GORM dialector based on database type
	var dialector gorm.Dialector
	switch config.Type {
	case "sqlite":
		dialector = sqlite.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	// Open database with GORM
	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Reduce log noise
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying SQL DB for database-specific configuration
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Enable foreign keys for SQLite (other databases have it enabled by default)
	if config.Type == "sqlite" {
		if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
		}
	}

	// Use provided cache or create a new one
	if cache == nil {
		cache = NewCache()
	}

	storage := &DB{
		DB:    gormDB,
		cache: cache,
	}

	// Migrate admin_users table to dashboard_users if it exists
	if err := storage.migrateAdminUsersToDashboardUsers(config.Type); err != nil {
		return nil, fmt.Errorf("failed to migrate admin_users table: %w", err)
	}

	// Run auto-migrations (works identically for all database types)
	if err := storage.autoMigrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Warm cache with MQTT users and ACL rules for performance
	if err := storage.warmCache(); err != nil {
		slog.Warn("Failed to warm cache", "error", err)
	}

	slog.Info("Database connected successfully", "type", config.Type)
	return storage, nil
}

// autoMigrate runs GORM's auto-migration for all models
func (db *DB) autoMigrate() error {
	return db.AutoMigrate(
		&DashboardUser{},
		&MQTTUser{},
		&MQTTClient{},
		&ACLRule{},
		&RetainedMessage{},
		&Bridge{},
		&BridgeTopic{},
		&Script{},
		&ScriptTrigger{},
		&ScriptLog{},
		&ScriptState{},
	)
}

// CreateDefaultAdmin creates a default admin user on first run
// Credentials are passed from the config (sourced from env vars, CLI flags, or defaults)
// Note: Like Grafana, these credentials ONLY work on first launch - once the admin user exists
// in the database, changing them has no effect
func (db *DB) CreateDefaultAdmin(adminUsername, adminPassword string) error {
	// Check if admin user already exists
	var existingAdmin DashboardUser
	err := db.Where("username = ?", adminUsername).First(&existingAdmin).Error
	if err == nil {
		// Admin already exists - env vars are ignored after first run (like Grafana)
		return nil
	}

	// Admin doesn't exist - create with env var credentials
	usingDefaults := adminUsername == "admin" && adminPassword == "admin"

	if usingDefaults {
		slog.Warn("Creating admin with default credentials (admin/admin)")
		slog.Warn("Set ADMIN_USERNAME and ADMIN_PASSWORD environment variables for production!")
		slog.Warn("Change the password immediately after first login!")
	} else {
		slog.Info("Creating admin user", "username", adminUsername)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := DashboardUser{
		Username:     adminUsername,
		PasswordHash: string(hash),
		Role:         "admin",
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	slog.Info("Admin user created successfully", "username", adminUsername)
	return nil
}

// migrateAdminUsersToDashboardUsers renames admin_users table to dashboard_users if it exists
// This is a one-time migration for the renamed table
func (db *DB) migrateAdminUsersToDashboardUsers(dbType string) error {
	// Check if admin_users table exists
	if db.Migrator().HasTable("admin_users") {
		slog.Info("Migrating admin_users table to dashboard_users")

		// Rename the table based on database type
		switch dbType {
		case "sqlite":
			return db.Exec("ALTER TABLE admin_users RENAME TO dashboard_users").Error
		case "postgres":
			return db.Exec("ALTER TABLE admin_users RENAME TO dashboard_users").Error
		case "mysql":
			return db.Exec("ALTER TABLE admin_users RENAME TO dashboard_users").Error
		default:
			return fmt.Errorf("unsupported database type for migration: %s", dbType)
		}
	}
	return nil
}

// Close closes the database connection and stops the cache cleanup goroutine
func (db *DB) Close() error {
	// Stop cache cleanup goroutine
	if db.cache != nil {
		db.cache.Stop()
	}

	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// warmCache pre-loads MQTT users and ACL rules into the cache for performance
// This prevents cache misses on startup and ensures the hot path is fast
func (db *DB) warmCache() error {
	// Load all MQTT users
	var users []MQTTUser
	if err := db.Find(&users).Error; err != nil {
		return fmt.Errorf("failed to load MQTT users for cache: %w", err)
	}
	db.cache.WarmMQTTUsers(users)
	slog.Info("Cache warmed with MQTT users", "count", len(users))

	// Load all ACL rules
	var rules []ACLRule
	if err := db.Find(&rules).Error; err != nil {
		return fmt.Errorf("failed to load ACL rules for cache: %w", err)
	}
	db.cache.WarmACLRules(rules)
	slog.Info("Cache warmed with ACL rules", "count", len(rules))

	return nil
}
