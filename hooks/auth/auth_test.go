package auth

import (
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// MockAuthenticator implements the Authenticator interface for testing
type MockAuthenticator struct {
	users map[string]string // username -> password
}

func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		users: make(map[string]string),
	}
}

func (m *MockAuthenticator) AddUser(username, password string) {
	m.users[username] = password
}

func (m *MockAuthenticator) AuthenticateUser(username, password string) (interface{}, error) {
	if storedPassword, ok := m.users[username]; ok {
		if storedPassword == password {
			return map[string]string{"username": username}, nil
		}
	}
	return nil, nil
}

func TestAuthHook_ID(t *testing.T) {
	auth := NewMockAuthenticator()
	hook := NewAuthHook(auth, true) // Allow anonymous for tests

	if hook.ID() != "database-auth" {
		t.Errorf("AuthHook.ID() = %v, want database-auth", hook.ID())
	}
}

func TestAuthHook_Provides(t *testing.T) {
	auth := NewMockAuthenticator()
	hook := NewAuthHook(auth, true) // Allow anonymous for tests

	tests := []struct {
		name     string
		hookType byte
		want     bool
	}{
		{
			name:     "provides OnConnectAuthenticate",
			hookType: mqtt.OnConnectAuthenticate,
			want:     true,
		},
		{
			name:     "provides OnConnect",
			hookType: mqtt.OnConnect,
			want:     true,
		},
		{
			name:     "does not provide OnDisconnect",
			hookType: mqtt.OnDisconnect,
			want:     false,
		},
		{
			name:     "does not provide OnPublish",
			hookType: mqtt.OnPublish,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hook.Provides(tt.hookType); got != tt.want {
				t.Errorf("AuthHook.Provides(%v) = %v, want %v", tt.hookType, got, tt.want)
			}
		})
	}
}

func TestAuthHook_OnConnectAuthenticate(t *testing.T) {
	auth := NewMockAuthenticator()
	auth.AddUser("validuser", "correctpassword")
	hook := NewAuthHook(auth, true) // Allow anonymous for this test

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{
			name:     "valid credentials",
			username: "validuser",
			password: "correctpassword",
			want:     true,
		},
		{
			name:     "invalid password",
			username: "validuser",
			password: "wrongpassword",
			want:     false,
		},
		{
			name:     "non-existent user",
			username: "nonexistent",
			password: "password123",
			want:     false,
		},
		{
			name:     "anonymous connection (empty username)",
			username: "",
			password: "",
			want:     true, // Anonymous connections are allowed
		},
		{
			name:     "empty password with username",
			username: "validuser",
			password: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock client
			cl := &mqtt.Client{
				ID: "test-client",
			}

			// Create a connect packet
			pk := packets.Packet{
				Connect: packets.ConnectParams{
					Username: []byte(tt.username),
					Password: []byte(tt.password),
				},
			}

			got := hook.OnConnectAuthenticate(cl, pk)
			if got != tt.want {
				t.Errorf("OnConnectAuthenticate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthHook_OnConnect(t *testing.T) {
	auth := NewMockAuthenticator()
	hook := NewAuthHook(auth, true) // Allow anonymous for this test

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "connection with username",
			username: "testuser",
			wantErr:  false,
		},
		{
			name:     "anonymous connection",
			username: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := &mqtt.Client{
				ID: "test-client",
			}

			pk := packets.Packet{
				Connect: packets.ConnectParams{
					Username: []byte(tt.username),
				},
			}

			err := hook.OnConnect(cl, pk)
			if (err != nil) != tt.wantErr {
				t.Errorf("OnConnect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
