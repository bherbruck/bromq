package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github/bherbruck/mqtt-server/internal/storage"
)

// === MQTT User (Credentials) Management Handlers ===

// ListMQTTUsers returns all MQTT credentials
func (h *Handler) ListMQTTUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListMQTTUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list MQTT credentials: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if users == nil {
		users = []storage.MQTTUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// CreateMQTTUser creates new MQTT credentials
func (h *Handler) CreateMQTTUser(w http.ResponseWriter, r *http.Request) {
	var req CreateMQTTUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	user, err := h.db.CreateMQTTUser(req.Username, req.Password, req.Description, req.Metadata)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create MQTT credentials: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// UpdateMQTTUser updates MQTT credentials information
func (h *Handler) UpdateMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials ID"}`, http.StatusBadRequest)
		return
	}

	// Check if user is provisioned from config
	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT credentials not found: %s"}`, err), http.StatusNotFound)
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
		http.Error(w, fmt.Sprintf(`{"error":"failed to update MQTT credentials: %s"}`, err), http.StatusInternalServerError)
		return
	}

	user, err = h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get MQTT credentials: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// DeleteMQTTUser deletes MQTT credentials
func (h *Handler) DeleteMQTTUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials ID"}`, http.StatusBadRequest)
		return
	}

	// Check if user is provisioned from config
	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT credentials not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if user.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot delete provisioned user. This user is managed by the configuration file. Remove it from the config file and restart the server to delete."}`, http.StatusConflict)
		return
	}

	if err := h.db.DeleteMQTTUser(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete MQTT credentials: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{Message: "MQTT credentials deleted"})
}

// UpdateMQTTUserPassword updates MQTT credentials password
func (h *Handler) UpdateMQTTUserPassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials ID"}`, http.StatusBadRequest)
		return
	}

	// Check if user is provisioned from config
	user, err := h.db.GetMQTTUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"MQTT credentials not found: %s"}`, err), http.StatusNotFound)
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

// ListMQTTClients returns all MQTT clients (connected devices)
func (h *Handler) ListMQTTClients(w http.ResponseWriter, r *http.Request) {
	// Check query parameter for active filter
	activeOnly := r.URL.Query().Get("active") == "true"

	clients, err := h.db.ListMQTTClients(activeOnly)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list MQTT clients: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if clients == nil {
		clients = []storage.MQTTClient{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
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
