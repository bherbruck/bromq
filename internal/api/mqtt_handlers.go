package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github/bherbruck/bromq/internal/storage"
)

// parsePaginationParams parses pagination parameters from request
func parsePaginationParams(r *http.Request) PaginationQuery {
	query := PaginationQuery{
		Page:     1,
		PageSize: 25, // Default page size
		Search:   "",
		SortBy:   "",
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

// ListMQTTUsers returns paginated MQTT users
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
	json.NewEncoder(w).Encode(response)
}

// CreateMQTTUser creates new MQTT user
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
	json.NewEncoder(w).Encode(user)
}

// GetMQTTUser returns a single MQTT user by ID
func (h *Handler) GetMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT user not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// UpdateMQTTUser updates MQTT user information
func (h *Handler) UpdateMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(user)
}

// DeleteMQTTUser deletes MQTT user
func (h *Handler) DeleteMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(SuccessResponse{Message: "MQTT user deleted"})
}

// UpdateMQTTUserPassword updates MQTT user password
func (h *Handler) UpdateMQTTUserPassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(SuccessResponse{Message: "password updated"})
}

// === MQTT Client Management Handlers ===

// ListMQTTClients returns paginated MQTT clients (connected devices)
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
	json.NewEncoder(w).Encode(response)
}

// GetMQTTClientDetails returns details about a specific MQTT client
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
	json.NewEncoder(w).Encode(client)
}

// UpdateMQTTClientMetadata updates a client's metadata
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
	json.NewEncoder(w).Encode(SuccessResponse{Message: "client metadata updated"})
}

// DeleteMQTTClient deletes a client record
func (h *Handler) DeleteMQTTClient(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid client ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteMQTTClient(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete client: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{Message: "client record deleted"})
}
