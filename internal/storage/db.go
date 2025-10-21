package storage

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	role TEXT DEFAULT 'user' CHECK(role IN ('user', 'admin')),
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS acl_rules (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER,
	topic_pattern TEXT NOT NULL,
	permission TEXT NOT NULL CHECK(permission IN ('pub', 'sub', 'pubsub')),
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS retained_messages (
	topic TEXT PRIMARY KEY,
	payload BLOB NOT NULL,
	qos INTEGER NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_acl_user_id ON acl_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_retained_topic ON retained_messages(topic);
`

type DB struct {
	*sql.DB
}

// Open creates a new database connection and initializes the schema
func Open(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	storage := &DB{db}

	// Create default admin user if not exists
	if err := storage.createDefaultAdmin(); err != nil {
		log.Printf("Warning: failed to create default admin: %v", err)
	}

	return storage, nil
}

// createDefaultAdmin creates a default admin user (username: admin, password: admin)
func (db *DB) createDefaultAdmin() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "admin").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Admin already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		"admin", string(hash), "admin",
	)
	if err != nil {
		return err
	}

	log.Println("Created default admin user (username: admin, password: admin)")
	log.Println("⚠️  Please change the default password immediately!")
	return nil
}
