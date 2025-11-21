package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateJWT(t *testing.T) {
	tests := []struct {
		name     string
		userID   uint
		username string
		role     string
		wantErr  bool
	}{
		{
			name:     "generate token for regular user",
			userID:   1,
			username: "testuser",
			role:     "user",
			wantErr:  false,
		},
		{
			name:     "generate token for admin",
			userID:   2,
			username: "admin",
			role:     "admin",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateJWT(testJWTSecret, tt.userID, tt.username, tt.role)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateJWT() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GenerateJWT() unexpected error: %v", err)
			}

			if token == "" {
				t.Errorf("GenerateJWT() returned empty token")
			}

			// Verify the token can be validated
			claims, err := ValidateJWT(testJWTSecret, token)
			if err != nil {
				t.Fatalf("ValidateJWT() failed: %v", err)
			}

			if claims.UserID != tt.userID {
				t.Errorf("ValidateJWT() userID = %v, want %v", claims.UserID, tt.userID)
			}

			if claims.Username != tt.username {
				t.Errorf("ValidateJWT() username = %v, want %v", claims.Username, tt.username)
			}

			if claims.Role != tt.role {
				t.Errorf("ValidateJWT() role = %v, want %v", claims.Role, tt.role)
			}
		})
	}
}

func TestValidateJWT(t *testing.T) {
	// Generate a valid token
	validToken, err := GenerateJWT(testJWTSecret, 1, "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Generate an expired token
	expiredClaims := JWTClaims{
		UserID:   1,
		Username: "testuser",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, _ := expiredToken.SignedString(testJWTSecret)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantErr: false,
		},
		{
			name:    "expired token",
			token:   expiredTokenString,
			wantErr: true,
		},
		{
			name:    "invalid token format",
			token:   "invalid.token.format",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "token with wrong signature",
			token:   validToken + "tampered",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ValidateJWT(testJWTSecret, tt.token)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateJWT() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateJWT() unexpected error: %v", err)
			}

			if claims == nil {
				t.Errorf("ValidateJWT() returned nil claims")
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Create test config
	testConfig := &Config{
		JWTSecret: string(testJWTSecret),
	}

	// Generate a valid token
	validToken, err := GenerateJWT(testJWTSecret, 1, "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Handler that should only be called if auth succeeds
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetUserFromContext(r)
		if !ok {
			t.Errorf("AuthMiddleware() claims not found in context")
		}
		if claims.Username != "testuser" {
			t.Errorf("AuthMiddleware() username = %v, want testuser", claims.Username)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
	}{
		{
			name:           "valid bearer token",
			authHeader:     fmt.Sprintf("Bearer %s", validToken),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid header format - no Bearer",
			authHeader:     validToken,
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid header format - wrong prefix",
			authHeader:     fmt.Sprintf("Basic %s", validToken),
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "empty token after Bearer",
			authHeader:     "Bearer ",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rec := httptest.NewRecorder()
			handler := NewAuthMiddleware(testConfig)(protectedHandler)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("AuthMiddleware() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	// Generate a valid token and create a request with claims in context
	validToken, err := GenerateJWT(testJWTSecret, 1, "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	claims, err := ValidateJWT(testJWTSecret, validToken)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	t.Run("get claims from context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(req.Context())

		// Add claims to context using the middleware's context key
		ctx := req.Context()
		ctx = contextWithValue(ctx, userContextKey, claims)
		req = req.WithContext(ctx)

		extractedClaims, ok := GetUserFromContext(req)
		if !ok {
			t.Errorf("GetUserFromContext() failed to extract claims")
		}

		if extractedClaims.Username != "testuser" {
			t.Errorf("GetUserFromContext() username = %v, want testuser", extractedClaims.Username)
		}
	})

	t.Run("no claims in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		_, ok := GetUserFromContext(req)
		if ok {
			t.Errorf("GetUserFromContext() should return false when no claims in context")
		}
	})
}

// Helper to add value to context (mimics what middleware does)
func contextWithValue(parent context.Context, key contextKey, val interface{}) context.Context {
	return context.WithValue(parent, key, val)
}

func TestCORSMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	tests := []struct {
		name           string
		method         string
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "regular request",
			method:         http.MethodGet,
			wantStatusCode: http.StatusOK,
			wantBody:       "success",
		},
		{
			name:           "preflight request",
			method:         http.MethodOptions,
			wantStatusCode: http.StatusOK,
			wantBody:       "",
		},
		{
			name:           "POST request",
			method:         http.MethodPost,
			wantStatusCode: http.StatusOK,
			wantBody:       "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			rec := httptest.NewRecorder()

			corsHandler := CORSMiddleware(handler)
			corsHandler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("CORSMiddleware() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}

			// Check CORS headers are set
			if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
				t.Errorf("CORSMiddleware() Access-Control-Allow-Origin = %v, want *", origin)
			}

			if methods := rec.Header().Get("Access-Control-Allow-Methods"); methods == "" {
				t.Errorf("CORSMiddleware() Access-Control-Allow-Methods not set")
			}

			if headers := rec.Header().Get("Access-Control-Allow-Headers"); headers == "" {
				t.Errorf("CORSMiddleware() Access-Control-Allow-Headers not set")
			}

			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("CORSMiddleware() body = %v, want %v", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestAdminOnly(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("admin access granted"))
	})

	tests := []struct {
		name           string
		setupContext   func(*http.Request) *http.Request
		wantStatusCode int
	}{
		{
			name: "admin user",
			setupContext: func(r *http.Request) *http.Request {
				claims := &JWTClaims{
					UserID:   1,
					Username: "admin",
					Role:     "admin",
				}
				ctx := context.WithValue(r.Context(), userContextKey, claims)
				return r.WithContext(ctx)
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "regular user",
			setupContext: func(r *http.Request) *http.Request {
				claims := &JWTClaims{
					UserID:   2,
					Username: "user",
					Role:     "user",
				}
				ctx := context.WithValue(r.Context(), userContextKey, claims)
				return r.WithContext(ctx)
			},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name: "no claims in context",
			setupContext: func(r *http.Request) *http.Request {
				return r
			},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			req = tt.setupContext(req)

			rec := httptest.NewRecorder()
			adminHandler := AdminOnly(handler)
			adminHandler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("AdminOnly() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusNotFound)

		if rw.statusCode != http.StatusNotFound {
			t.Errorf("responseWriter.statusCode = %v, want %v", rw.statusCode, http.StatusNotFound)
		}

		if rec.Code != http.StatusNotFound {
			t.Errorf("underlying ResponseWriter.Code = %v, want %v", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("default status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		// Write without calling WriteHeader
		rw.Write([]byte("test"))

		// Status code should remain OK
		if rw.statusCode != http.StatusOK {
			t.Errorf("responseWriter.statusCode = %v, want %v", rw.statusCode, http.StatusOK)
		}
	})
}
