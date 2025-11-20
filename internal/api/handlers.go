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

// Login godoc
// @Summary Login to dashboard
// @Description Authenticate with dashboard credentials and receive JWT token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Invalid credentials"
// @Failure 500 {object} ErrorResponse
// @Router /auth/login [post]
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

	token, err := GenerateJWT(h.config.JWTSecret, user.ID, user.Username, user.Role)
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

// ListACL godoc
// @Summary List ACL rules
// @Description Get paginated list of access control rules
// @Tags ACL
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param search query string false "Search by topic"
// @Param sortBy query string false "Sort field" default(id)
// @Param sortOrder query string false "Sort order (asc/desc)" default(asc)
// @Success 200 {object} PaginatedResponse{data=[]storage.ACLRule}
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /acl [get]
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

// CreateACL godoc
// @Summary Create ACL rule
// @Description Create a new access control rule for an MQTT user
// @Tags ACL
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rule body CreateACLRequest true "ACL rule details"
// @Success 201 {object} storage.ACLRule
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 409 {object} ErrorResponse "Duplicate rule"
// @Failure 500 {object} ErrorResponse
// @Router /acl [post]
func (h *Handler) CreateACL(w http.ResponseWriter, r *http.Request) {
	var req CreateACLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	rule, err := h.db.CreateACLRule(req.MQTTUserID, req.Topic, req.Permission)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rule)
}

// UpdateACL godoc
// @Summary Update ACL rule
// @Description Update an existing access control rule
// @Tags ACL
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ACL Rule ID"
// @Param rule body UpdateACLRequest true "Updated ACL rule details"
// @Success 200 {object} storage.ACLRule
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse "Rule not found"
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be modified"
// @Failure 500 {object} ErrorResponse
// @Router /acl/{id} [put]
func (h *Handler) UpdateACL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid ACL rule ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

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

	rule, err := h.db.UpdateACLRule(id, req.Topic, req.Permission)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update ACL rule: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rule)
}

// DeleteACL godoc
// @Summary Delete ACL rule
// @Description Delete an access control rule
// @Tags ACL
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ACL Rule ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse "Rule not found"
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be deleted"
// @Failure 500 {object} ErrorResponse
// @Router /acl/{id} [delete]
func (h *Handler) DeleteACL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid ACL rule ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

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

// ListClients godoc
// @Summary List connected clients
// @Description Get list of all currently connected MQTT clients with their connection details
// @Tags Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} object
// @Failure 401 {object} ErrorResponse
// @Router /clients [get]
func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	clients := h.mqtt.GetClients()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(clients)
}

// GetClientDetails godoc
// @Summary Get client details
// @Description Get detailed information about a specific connected MQTT client by client ID
// @Tags Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Client ID"
// @Success 200 {object} object
// @Failure 400 {object} ErrorResponse "Client ID required"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Client not found"
// @Router /clients/{id} [get]
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

// DisconnectClient godoc
// @Summary Disconnect client
// @Description Forcefully disconnect a connected MQTT client by client ID
// @Tags Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Client ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Client ID required"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /clients/{id} [delete]
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

// GetMetrics godoc
// @Summary Get server metrics
// @Description Get MQTT server metrics in JSON format including clients, messages, subscriptions, and system stats
// @Tags Metrics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object
// @Failure 401 {object} ErrorResponse
// @Router /metrics [get]
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.mqtt.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}
