package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github/bherbruck/mqtt-server/internal/mqtt"
	"github/bherbruck/mqtt-server/internal/storage"
)

// Handler holds dependencies for API handlers
type Handler struct {
	db   *storage.DB
	mqtt *mqtt.Server
}

// NewHandler creates a new API handler
func NewHandler(db *storage.DB, mqttServer *mqtt.Server) *Handler {
	return &Handler{
		db:   db,
		mqtt: mqttServer,
	}
}

// Login handles user authentication and returns a JWT token
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	userInterface, err := h.db.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"authentication error: %s"}`, err), http.StatusInternalServerError)
		return
	}

	if userInterface == nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	user, ok := userInterface.(*storage.User)
	if !ok {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	token, err := GenerateJWT(user.ID, user.Username, user.Role)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to generate token: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: token,
		User:  user,
	})
}

// ListUsers returns all users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list users: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if users == nil {
		users = []storage.User{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// CreateUser creates a new user
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	user, err := h.db.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// UpdateUser updates a user's information
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateUser(id, req.Username, req.Role); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	user, err := h.db.GetUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// DeleteUser deletes a user
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteUser(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{Message: "user deleted"})
}

// ListACL returns all ACL rules
func (h *Handler) ListACL(w http.ResponseWriter, r *http.Request) {
	rules, err := h.db.ListACLRules()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list ACL rules: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if rules == nil {
		rules = []storage.ACLRule{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

// CreateACL creates a new ACL rule
func (h *Handler) CreateACL(w http.ResponseWriter, r *http.Request) {
	var req CreateACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	rule, err := h.db.CreateACLRule(req.UserID, req.TopicPattern, req.Permission)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// DeleteACL deletes an ACL rule
func (h *Handler) DeleteACL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid ACL rule ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteACLRule(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{Message: "ACL rule deleted"})
}

// ListClients returns all connected MQTT clients
func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	clients := h.mqtt.GetClients()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// GetClientDetails returns detailed information about a specific client
func (h *Handler) GetClientDetails(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	if clientID == "" {
		http.Error(w, `{"error":"client ID required"}`, http.StatusBadRequest)
		return
	}

	details, err := h.mqtt.GetClientDetails(clientID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

// DisconnectClient forcefully disconnects an MQTT client
func (h *Handler) DisconnectClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	if clientID == "" {
		http.Error(w, `{"error":"client ID required"}`, http.StatusBadRequest)
		return
	}

	if err := h.mqtt.DisconnectClient(clientID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to disconnect client: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{Message: "client disconnected"})
}

// GetMetrics returns server metrics
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.mqtt.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
