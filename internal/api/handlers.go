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
// Only DashboardUsers can log in to the dashboard
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Authenticate against DashboardUser table only
	user, err := h.db.AuthenticateDashboardUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"authentication error: %s"}`, err), http.StatusInternalServerError)
		return
	}

	if user == nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := GenerateJWT(int(user.ID), user.Username, user.Role)
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

	rule, err := h.db.CreateACLRule(req.MQTTUserID, req.TopicPattern, req.Permission)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// UpdateACL updates an existing ACL rule
func (h *Handler) UpdateACL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid ACL rule ID"}`, http.StatusBadRequest)
		return
	}

	var req UpdateACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	rule, err := h.db.UpdateACLRule(id, req.TopicPattern, req.Permission)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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
