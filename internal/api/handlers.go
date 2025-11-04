package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github/bherbruck/bromq/internal/mqtt"
	"github/bherbruck/bromq/internal/script"
	"github/bherbruck/bromq/internal/storage"
)

// Handler holds dependencies for API handlers
type Handler struct {
	db     *storage.DB
	mqtt   *mqtt.Server
	engine *script.Engine
	config *Config
}

// NewHandler creates a new API handler
func NewHandler(db *storage.DB, mqttServer *mqtt.Server, scriptEngine *script.Engine, config *Config) *Handler {
	return &Handler{
		db:     db,
		mqtt:   mqttServer,
		engine: scriptEngine,
		config: config,
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

	token, err := GenerateJWT(h.config.JWTSecret, int(user.ID), user.Username, user.Role)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to generate token: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{
		Token: token,
		User:  user,
	})
}

// ListACL returns paginated ACL rules
func (h *Handler) ListACL(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	params := parsePaginationParams(r)

	// Get paginated rules
	rules, total, err := h.db.ListACLRulesPaginated(params.Page, params.PageSize, params.Search, params.SortBy, params.SortOrder)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list ACL rules: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if rules == nil {
		rules = []storage.ACLRule{}
	}

	// Calculate total pages
	totalPages := 0
	if params.PageSize > 0 {
		totalPages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}

	// Build paginated response
	response := PaginatedResponse{
		Data: rules,
		Pagination: PaginationMetadata{
			Total:      total,
			Page:       params.Page,
			PageSize:   params.PageSize,
			TotalPages: totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
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
	_ = json.NewEncoder(w).Encode(rule)
}

// UpdateACL updates an existing ACL rule
func (h *Handler) UpdateACL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid ACL rule ID"}`, http.StatusBadRequest)
		return
	}

	// Check if ACL rule is provisioned from config
	existingRule, err := h.db.GetACLRule(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"ACL rule not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if existingRule.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot modify provisioned ACL rule. This rule is managed by the configuration file. Edit the config file and restart the server to make changes."}`, http.StatusConflict)
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
	_ = json.NewEncoder(w).Encode(rule)
}

// DeleteACL deletes an ACL rule
func (h *Handler) DeleteACL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid ACL rule ID"}`, http.StatusBadRequest)
		return
	}

	// Check if ACL rule is provisioned from config
	existingRule, err := h.db.GetACLRule(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"ACL rule not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if existingRule.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot delete provisioned ACL rule. This rule is managed by the configuration file. Remove it from the config file and restart the server to delete."}`, http.StatusConflict)
		return
	}

	if err := h.db.DeleteACLRule(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "ACL rule deleted"})
}

// ListClients returns all connected MQTT clients
func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	clients := h.mqtt.GetClients()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(clients)
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
	_ = json.NewEncoder(w).Encode(details)
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
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "client disconnected"})
}

// GetMetrics returns server metrics
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.mqtt.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}
