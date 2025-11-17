package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github/bherbruck/bromq/internal/storage"
	"gorm.io/datatypes"
)

// === Bridge Management Handlers ===

// ListBridges returns paginated bridges
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

// GetBridge returns a single bridge by ID
func (h *Handler) GetBridge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid bridge ID"}`, http.StatusBadRequest)
		return
	}

	bridge, err := h.db.GetBridge(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bridge not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bridge)
}

// CreateBridge creates a new bridge
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
			Local:  t.Local,
			Remote: t.Remote,
			Direction:     t.Direction,
			QoS:           t.QoS,
		}
	}

	// Create bridge
	bridge, err := h.db.CreateBridge(
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

// UpdateBridge updates a bridge's configuration
func (h *Handler) UpdateBridge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid bridge ID"}`, http.StatusBadRequest)
		return
	}

	// Check if bridge is provisioned from config
	bridge, err := h.db.GetBridge(uint(id))
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
		uint(id),
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
			BridgeID:      uint(id),
			Local:  t.Local,
			Remote: t.Remote,
			Direction:     t.Direction,
			QoS:           t.QoS,
		}
	}

	if err := h.db.UpdateBridgeTopics(uint(id), topics); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update bridge topics: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Fetch updated bridge
	bridge, err = h.db.GetBridge(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get updated bridge: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bridge)
}

// DeleteBridge deletes a bridge
func (h *Handler) DeleteBridge(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid bridge ID"}`, http.StatusBadRequest)
		return
	}

	// Check if bridge is provisioned from config
	bridge, err := h.db.GetBridge(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bridge not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if bridge.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot delete provisioned bridge. This bridge is managed by the configuration file. Remove it from the config file and restart the server to delete."}`, http.StatusConflict)
		return
	}

	if err := h.db.DeleteBridge(uint(id)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete bridge: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "bridge deleted"})
}
