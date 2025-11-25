package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github/bromq-dev/bromq/internal/storage"

	"gorm.io/datatypes"
)

// === Bridge Management Handlers ===

// ListBridges godoc
// @Summary List bridges
// @Description Get paginated list of MQTT bridges with their topic mappings
// @Tags Bridges
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param search query string false "Search by bridge name"
// @Param sortBy query string false "Sort field" default(id)
// @Param sortOrder query string false "Sort order (asc/desc)" default(asc)
// @Success 200 {object} PaginatedResponse{data=[]storage.Bridge}
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /bridges [get]
func (h *Handler) ListBridges(w http.ResponseWriter, r *http.Request) {
	params := parsePaginationParams(r)

	bridges, total, err := h.db.ListBridgesPaginated(params.Page, params.PageSize, params.Search, params.SortBy, params.SortOrder)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list bridges: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if bridges == nil {
		bridges = []storage.Bridge{}
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	response := PaginatedResponse{
		Data: bridges,
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

// GetBridge godoc
// @Summary Get bridge
// @Description Get a single MQTT bridge by ID with its topic mappings
// @Tags Bridges
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bridge ID"
// @Success 200 {object} storage.Bridge
// @Failure 400 {object} ErrorResponse "Invalid bridge ID"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Bridge not found"
// @Router /bridges/{id} [get]
func (h *Handler) GetBridge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid bridge ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	bridge, err := h.db.GetBridge(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bridge not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bridge)
}

// CreateBridge godoc
// @Summary Create bridge
// @Description Create a new MQTT bridge with topic mappings to forward messages to/from remote brokers
// @Tags Bridges
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param bridge body CreateBridgeRequest true "Bridge configuration with topics"
// @Success 201 {object} storage.Bridge
// @Failure 400 {object} ErrorResponse "Invalid request or validation error"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /bridges [post]
func (h *Handler) CreateBridge(w http.ResponseWriter, r *http.Request) {
	var req CreateBridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, `{"error":"bridge name is required"}`, http.StatusBadRequest)
		return
	}
	if req.Host == "" {
		http.Error(w, `{"error":"remote host is required"}`, http.StatusBadRequest)
		return
	}

	// Validate topics
	for i, topic := range req.Topics {
		if topic.Local == "" {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: local_pattern is required"}`, i), http.StatusBadRequest)
			return
		}
		if topic.Remote == "" {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: remote_pattern is required"}`, i), http.StatusBadRequest)
			return
		}
		if topic.Direction != "in" && topic.Direction != "out" && topic.Direction != "both" {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: direction must be 'in', 'out', or 'both'"}`, i), http.StatusBadRequest)
			return
		}
		if topic.QoS > 2 {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: QoS must be 0, 1, or 2"}`, i), http.StatusBadRequest)
			return
		}
	}

	// Convert metadata to JSON
	var metadata datatypes.JSON
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid metadata: %s"}`, err), http.StatusBadRequest)
			return
		}
		metadata = metadataBytes
	}

	// Convert topic requests to storage topics
	topics := make([]storage.BridgeTopic, len(req.Topics))
	for i, t := range req.Topics {
		topics[i] = storage.BridgeTopic{
			Local:     t.Local,
			Remote:    t.Remote,
			Direction: t.Direction,
			QoS:       t.QoS,
		}
	}

	// Set default MQTT version if not specified
	mqttVersion := req.MQTTVersion
	if mqttVersion == "" {
		mqttVersion = "5" // Default to MQTT v5
	}

	// Create bridge
	bridge, err := h.db.CreateBridge(
		req.Name,
		req.Host,
		req.Port,
		req.Username,
		req.Password,
		req.ClientID,
		mqttVersion,
		req.CleanSession,
		req.KeepAlive,
		req.ConnectionTimeout,
		metadata,
		topics,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create bridge: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(bridge)
}

// UpdateBridge godoc
// @Summary Update bridge
// @Description Update an existing MQTT bridge configuration and topic mappings
// @Tags Bridges
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bridge ID"
// @Param bridge body UpdateBridgeRequest true "Updated bridge configuration"
// @Success 200 {object} storage.Bridge
// @Failure 400 {object} ErrorResponse "Invalid bridge ID or validation error"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse "Bridge not found"
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be modified"
// @Failure 500 {object} ErrorResponse
// @Router /bridges/{id} [put]
func (h *Handler) UpdateBridge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid bridge ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	// Check if bridge is provisioned from config
	bridge, err := h.db.GetBridge(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bridge not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if bridge.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot modify provisioned bridge. This bridge is managed by the configuration file. Edit the config file and restart the server to make changes."}`, http.StatusConflict)
		return
	}

	var req UpdateBridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, `{"error":"bridge name is required"}`, http.StatusBadRequest)
		return
	}
	if req.Host == "" {
		http.Error(w, `{"error":"remote host is required"}`, http.StatusBadRequest)
		return
	}

	// Validate topics
	for i, topic := range req.Topics {
		if topic.Local == "" {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: local_pattern is required"}`, i), http.StatusBadRequest)
			return
		}
		if topic.Remote == "" {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: remote_pattern is required"}`, i), http.StatusBadRequest)
			return
		}
		if topic.Direction != "in" && topic.Direction != "out" && topic.Direction != "both" {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: direction must be 'in', 'out', or 'both'"}`, i), http.StatusBadRequest)
			return
		}
		if topic.QoS > 2 {
			http.Error(w, fmt.Sprintf(`{"error":"topic %d: QoS must be 0, 1, or 2"}`, i), http.StatusBadRequest)
			return
		}
	}

	// Convert metadata to JSON
	var metadata datatypes.JSON
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid metadata: %s"}`, err), http.StatusBadRequest)
			return
		}
		metadata = metadataBytes
	}

	// Update bridge basic info
	if _, err := h.db.UpdateBridge(
		id,
		req.Name,
		req.Host,
		req.Port,
		req.Username,
		req.Password,
		req.ClientID,
		req.CleanSession,
		req.KeepAlive,
		req.ConnectionTimeout,
		metadata,
	); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update bridge: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Update topics
	topics := make([]storage.BridgeTopic, len(req.Topics))
	for i, t := range req.Topics {
		topics[i] = storage.BridgeTopic{
			BridgeID:  id,
			Local:     t.Local,
			Remote:    t.Remote,
			Direction: t.Direction,
			QoS:       t.QoS,
		}
	}

	if err := h.db.UpdateBridgeTopics(uint(id), topics); err != nil { // #nosec G115 -- id from route param, validated positive
		http.Error(w, fmt.Sprintf(`{"error":"failed to update bridge topics: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Fetch updated bridge
	bridge, err = h.db.GetBridge(uint(id)) // #nosec G115 -- id from route param, validated positive
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get updated bridge: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bridge)
}

// DeleteBridge godoc
// @Summary Delete bridge
// @Description Delete an MQTT bridge and all its topic mappings
// @Tags Bridges
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bridge ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Invalid bridge ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse "Bridge not found"
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be deleted"
// @Failure 500 {object} ErrorResponse
// @Router /bridges/{id} [delete]
func (h *Handler) DeleteBridge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid bridge ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	// Check if bridge is provisioned from config
	bridge, err := h.db.GetBridge(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bridge not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if bridge.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot delete provisioned bridge. This bridge is managed by the configuration file. Remove it from the config file and restart the server to delete."}`, http.StatusConflict)
		return
	}

	if err := h.db.DeleteBridge(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete bridge: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "bridge deleted"})
}
