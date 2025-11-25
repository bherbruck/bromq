package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github/bromq-dev/bromq/internal/storage"
)

// parsePaginationParams parses pagination parameters from request
func parsePaginationParams(r *http.Request) PaginationQuery {
	query := PaginationQuery{
		Page:      1,
		PageSize:  25, // Default page size
		Search:    "",
		SortBy:    "",
		SortOrder: "desc",
	}

	if page := r.URL.Query().Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			query.Page = p
		}
	}

	if pageSize := r.URL.Query().Get("pageSize"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil && ps > 0 && ps <= 100 {
			query.PageSize = ps
		}
	}

	query.Search = r.URL.Query().Get("search")
	query.SortBy = r.URL.Query().Get("sortBy")

	if sortOrder := r.URL.Query().Get("sortOrder"); sortOrder == "asc" || sortOrder == "desc" {
		query.SortOrder = sortOrder
	}

	return query
}

// === MQTT User (Credentials) Management Handlers ===

// ListMQTTUsers godoc
// @Summary List MQTT users
// @Description Get paginated list of MQTT credentials (shared by devices)
// @Tags MQTT Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param search query string false "Search by username"
// @Param sortBy query string false "Sort field" default(id)
// @Param sortOrder query string false "Sort order (asc/desc)" default(asc)
// @Success 200 {object} PaginatedResponse{data=[]storage.MQTTUser}
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/users [get]
func (h *Handler) ListMQTTUsers(w http.ResponseWriter, r *http.Request) {
	params := parsePaginationParams(r)

	users, total, err := h.db.ListMQTTUsersPaginated(params.Page, params.PageSize, params.Search, params.SortBy, params.SortOrder)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list MQTT users: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if users == nil {
		users = []storage.MQTTUser{}
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	response := PaginatedResponse{
		Data: users,
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

// CreateMQTTUser godoc
// @Summary Create MQTT user
// @Description Create new MQTT credentials (can be shared by multiple devices)
// @Tags MQTT Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body CreateMQTTUserRequest true "MQTT user details"
// @Success 201 {object} storage.MQTTUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/users [post]
func (h *Handler) CreateMQTTUser(w http.ResponseWriter, r *http.Request) {
	var req CreateMQTTUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	user, err := h.db.CreateMQTTUser(req.Username, req.Password, req.Description, req.Metadata)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create MQTT user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// GetMQTTUser godoc
// @Summary Get MQTT user
// @Description Get a single MQTT user by ID
// @Tags MQTT Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MQTT User ID"
// @Success 200 {object} storage.MQTTUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /mqtt/users/{id} [get]
func (h *Handler) GetMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT user not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// UpdateMQTTUser godoc
// @Summary Update MQTT user
// @Description Update MQTT user credentials
// @Tags MQTT Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MQTT User ID"
// @Param user body UpdateMQTTUserRequest true "Updated MQTT user details"
// @Success 200 {object} storage.MQTTUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be modified"
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/users/{id} [put]
func (h *Handler) UpdateMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	// Check if user is provisioned from config
	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT user not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if user.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot modify provisioned user. This user is managed by the configuration file. Edit the config file and restart the server to make changes."}`, http.StatusConflict)
		return
	}

	var req UpdateMQTTUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateMQTTUser(id, req.Username, req.Description, req.Metadata); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update MQTT user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	user, err = h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get MQTT user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// DeleteMQTTUser godoc
// @Summary Delete MQTT user
// @Description Delete MQTT credentials (also deletes associated clients and ACL rules)
// @Tags MQTT Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MQTT User ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be deleted"
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/users/{id} [delete]
func (h *Handler) DeleteMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	// Check if user is provisioned from config
	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT user not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if user.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot delete provisioned user. This user is managed by the configuration file. Remove it from the config file and restart the server to delete."}`, http.StatusConflict)
		return
	}

	if err := h.db.DeleteMQTTUser(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete MQTT user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "MQTT user deleted"})
}

// UpdateMQTTUserPassword godoc
// @Summary Update MQTT user password
// @Description Update password for MQTT credentials
// @Tags MQTT Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MQTT User ID"
// @Param password body UpdateMQTTPasswordRequest true "New password"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be modified"
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/users/{id}/password [put]
func (h *Handler) UpdateMQTTUserPassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	// Check if user is provisioned from config
	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT user not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if user.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot modify provisioned user password. This user is managed by the configuration file. Edit the config file and restart the server to make changes."}`, http.StatusConflict)
		return
	}

	var req UpdateMQTTPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, `{"error":"password cannot be empty"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateMQTTUserPassword(id, req.Password); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update password: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "password updated"})
}

// === MQTT Client Management Handlers ===

// ListMQTTClients godoc
// @Summary List MQTT clients
// @Description Get paginated list of connected MQTT devices/clients
// @Tags MQTT Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param search query string false "Search by client ID"
// @Param sortBy query string false "Sort field" default(id)
// @Param sortOrder query string false "Sort order (asc/desc)" default(asc)
// @Param active query boolean false "Filter active clients only"
// @Success 200 {object} PaginatedResponse{data=[]storage.MQTTClient}
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/clients [get]
func (h *Handler) ListMQTTClients(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	params := parsePaginationParams(r)

	// Check query parameter for active filter
	activeOnly := r.URL.Query().Get("active") == "true"

	// Get paginated clients
	clients, total, err := h.db.ListMQTTClientsPaginated(params.Page, params.PageSize, params.Search, params.SortBy, params.SortOrder, activeOnly)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list MQTT clients: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if clients == nil {
		clients = []storage.MQTTClient{}
	}

	// Calculate total pages
	totalPages := 0
	if params.PageSize > 0 {
		totalPages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}

	// Build paginated response
	response := PaginatedResponse{
		Data: clients,
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

// GetMQTTClientDetails godoc
// @Summary Get MQTT client details
// @Description Get details for a specific MQTT client by client ID
// @Tags MQTT Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param client_id path string true "Client ID"
// @Success 200 {object} storage.MQTTClient
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /mqtt/clients/{client_id} [get]
func (h *Handler) GetMQTTClientDetails(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	if clientID == "" {
		http.Error(w, `{"error":"client_id is required"}`, http.StatusBadRequest)
		return
	}

	client, err := h.db.GetMQTTClientByClientID(clientID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"client not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(client)
}

// UpdateMQTTClientMetadata godoc
// @Summary Update MQTT client metadata
// @Description Update custom metadata for an MQTT client
// @Tags MQTT Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param client_id path string true "Client ID"
// @Param metadata body UpdateMQTTClientMetadataRequest true "Client metadata"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/clients/{client_id}/metadata [put]
func (h *Handler) UpdateMQTTClientMetadata(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	if clientID == "" {
		http.Error(w, `{"error":"client_id is required"}`, http.StatusBadRequest)
		return
	}

	var req UpdateMQTTClientMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateMQTTClientMetadata(clientID, req.Metadata); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update client metadata: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "client metadata updated"})
}

// DeleteMQTTClient godoc
// @Summary Delete MQTT client
// @Description Delete an MQTT client record from tracking
// @Tags MQTT Clients
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Client ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /mqtt/clients/{id} [delete]
func (h *Handler) DeleteMQTTClient(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid client ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	if err := h.db.DeleteMQTTClient(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete client: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "client record deleted"})
}
