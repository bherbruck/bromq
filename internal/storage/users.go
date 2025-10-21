package storage

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never expose password hash in JSON
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetUser retrieves a user by ID
func (db *DB) GetUser(id int) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetUserByUsername retrieves a user by username
func (db *DB) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// ListUsers returns all users
func (db *DB) ListUsers() ([]User, error) {
	rows, err := db.Query("SELECT id, username, password_hash, role, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// CreateUser creates a new user with a hashed password
func (db *DB) CreateUser(username, password, role string) (*User, error) {
	// Validate role
	if role != "user" && role != "admin" {
		return nil, fmt.Errorf("invalid role: must be 'user' or 'admin'")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert user
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		username, string(hash), role,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	return db.GetUser(int(id))
}

// UpdateUser updates a user's information
func (db *DB) UpdateUser(id int, username, role string) error {
	// Validate role
	if role != "user" && role != "admin" {
		return fmt.Errorf("invalid role: must be 'user' or 'admin'")
	}

	result, err := db.Exec(
		"UPDATE users SET username = ?, role = ? WHERE id = ?",
		username, role, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateUserPassword updates a user's password
func (db *DB) UpdateUserPassword(id int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	result, err := db.Exec(
		"UPDATE users SET password_hash = ? WHERE id = ?",
		string(hash), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user by ID
func (db *DB) DeleteUser(id int) error {
	result, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// AuthenticateUser verifies username and password, returns user if valid
func (db *DB) AuthenticateUser(username, password string) (interface{}, error) {
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil // User not found
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, nil // Invalid password
	}

	return user, nil
}
