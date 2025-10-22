package storage

import (
	"fmt"
	"log"

	sqlite "github.com/glebarez/sqlite" // Pure Go SQLite driver (no CGO required)
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB wraps the GORM database connection
type DB struct {
	*gorm.DB
}

// Open creates a new database connection and runs auto-migrations
// Supports SQLite, PostgreSQL, and MySQL based on the provided configuration
func Open(config *DatabaseConfig) (*DB, error) {
	if config == nil {
		// Default to SQLite for backward compatibility
		config = DefaultSQLiteConfig("mqtt-server.db")
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

	storage := &DB{gormDB}

	// Migrate admin_users table to dashboard_users if it exists
	if err := storage.migrateAdminUsersToDashboardUsers(config.Type); err != nil {
		return nil, fmt.Errorf("failed to migrate admin_users table: %w", err)
	}

	// Run auto-migrations (works identically for all database types)
	if err := storage.autoMigrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create default admin user if not exists
	if err := storage.createDefaultAdmin(); err != nil {
		log.Printf("Warning: failed to create default admin: %v", err)
	}

	log.Printf("Database connected successfully (type: %s)", config.Type)
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
	)
}

// createDefaultAdmin creates a default admin user on first run
// Credentials can be configured via ADMIN_USERNAME and ADMIN_PASSWORD environment variables
// Defaults to "admin"/"admin" if not set (for development convenience)
// Note: Like Grafana, these env vars ONLY work on first launch - once the admin user exists
// in the database, changing the env vars has no effect
func (db *DB) createDefaultAdmin() error {
	// Get admin credentials from environment (only used on first run)
	adminUsername := getEnv("ADMIN_USERNAME", "admin")
	adminPassword := getEnv("ADMIN_PASSWORD", "admin")

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
		log.Println("⚠️  WARNING: Creating admin with default credentials (admin/admin)")
		log.Println("⚠️  Set ADMIN_USERNAME and ADMIN_PASSWORD environment variables for production!")
		log.Println("⚠️  Change the password immediately after first login!")
	} else {
		log.Printf("Creating admin user: %s", adminUsername)
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

	log.Printf("✓ Admin user created successfully: %s", adminUsername)
	return nil
}

// migrateAdminUsersToDashboardUsers renames admin_users table to dashboard_users if it exists
// This is a one-time migration for the renamed table
func (db *DB) migrateAdminUsersToDashboardUsers(dbType string) error {
	// Check if admin_users table exists
	if db.Migrator().HasTable("admin_users") {
		log.Println("Migrating admin_users table to dashboard_users...")

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

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
