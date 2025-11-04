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

// === Script Management Handlers ===

// ListScripts returns paginated scripts
func (h *Handler) ListScripts(w http.ResponseWriter, r *http.Request) {
	params := parsePaginationParams(r)

	scripts, total, err := h.db.ListScriptsPaginated(params.Page, params.PageSize, params.Search, params.SortBy, params.SortOrder)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list scripts: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if scripts == nil {
		scripts = []storage.Script{}
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	response := PaginatedResponse{
		Data: scripts,
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

// GetScript returns a single script by ID
func (h *Handler) GetScript(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	script, err := h.db.GetScript(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"script not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(script)
}

// CreateScript creates a new script
func (h *Handler) CreateScript(w http.ResponseWriter, r *http.Request) {
	var req CreateScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, `{"error":"script name is required"}`, http.StatusBadRequest)
		return
	}
	if req.ScriptContent == "" {
		http.Error(w, `{"error":"script content is required"}`, http.StatusBadRequest)
		return
	}

	// Convert metadata to JSON
	var metadata datatypes.JSON
	if req.Metadata != nil {
		metaBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid metadata: %s"}`, err), http.StatusBadRequest)
			return
		}
		metadata = datatypes.JSON(metaBytes)
	}

	// Convert triggers
	triggers := make([]storage.ScriptTrigger, len(req.Triggers))
	for i, t := range req.Triggers {
		triggers[i] = storage.ScriptTrigger{
			TriggerType: t.TriggerType,
			TopicFilter: t.TopicFilter,
			Priority:    t.Priority,
			Enabled:     t.Enabled,
		}
	}

	script, err := h.db.CreateScript(req.Name, req.Description, req.ScriptContent, req.Enabled, metadata, triggers)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create script: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(script)
}

// UpdateScript updates a script
func (h *Handler) UpdateScript(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	// Check if script is provisioned from config
	script, err := h.db.GetScript(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"script not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if script.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot modify provisioned script. This script is managed by the configuration file. Edit the config file and restart the server to make changes."}`, http.StatusConflict)
		return
	}

	var req UpdateScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Convert metadata to JSON
	var metadata datatypes.JSON
	if req.Metadata != nil {
		metaBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid metadata: %s"}`, err), http.StatusBadRequest)
			return
		}
		metadata = datatypes.JSON(metaBytes)
	}

	// Convert triggers
	triggers := make([]storage.ScriptTrigger, len(req.Triggers))
	for i, t := range req.Triggers {
		triggers[i] = storage.ScriptTrigger{
			TriggerType: t.TriggerType,
			TopicFilter: t.TopicFilter,
			Priority:    t.Priority,
			Enabled:     t.Enabled,
		}
	}

	if err := h.db.UpdateScript(uint(id), req.Name, req.Description, req.ScriptContent, req.Enabled, metadata, triggers); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update script: %s"}`, err), http.StatusInternalServerError)
		return
	}

	script, err = h.db.GetScript(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get updated script: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(script)
}

// DeleteScript deletes a script
func (h *Handler) DeleteScript(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	// Check if script is provisioned from config
	script, err := h.db.GetScript(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"script not found: %s"}`, err), http.StatusNotFound)
		return
	}

	if script.ProvisionedFromConfig {
		http.Error(w, `{"error":"Cannot delete provisioned script. This script is managed by the configuration file. Remove it from the config file and restart the server to delete it."}`, http.StatusConflict)
		return
	}

	if err := h.db.DeleteScript(uint(id)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete script: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "script deleted successfully"})
}

// EnableScript toggles script enabled status
func (h *Handler) EnableScript(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateScriptEnabled(uint(id), req.Enabled); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update script: %s"}`, err), http.StatusInternalServerError)
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: fmt.Sprintf("script %s successfully", status)})
}

// TestScript tests a script with mock event data
func (h *Handler) TestScript(w http.ResponseWriter, r *http.Request) {
	var req TestScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ScriptContent == "" {
		http.Error(w, `{"error":"script content is required"}`, http.StatusBadRequest)
		return
	}
	if req.TriggerType == "" {
		http.Error(w, `{"error":"trigger type is required"}`, http.StatusBadRequest)
		return
	}

	// Test the script
	result := h.engine.TestScript(req.ScriptContent, req.TriggerType, req.EventData)

	response := map[string]interface{}{
		"success":           result.Success,
		"execution_time_ms": result.ExecutionTimeMs,
		"logs":              result.Logs,
	}

	if result.Error != nil {
		response["error"] = result.Error.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// GetScriptLogs returns logs for a script
func (h *Handler) GetScriptLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	params := parsePaginationParams(r)
	level := r.URL.Query().Get("level") // Optional filter by level

	logs, total, err := h.db.ListScriptLogs(uint(id), params.Page, params.PageSize, level)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list logs: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if logs == nil {
		logs = []storage.ScriptLog{}
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	response := PaginatedResponse{
		Data: logs,
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

// ClearScriptLogs clears all logs for a script
func (h *Handler) ClearScriptLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.ClearScriptLogs(uint(id)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to clear logs: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "logs cleared successfully"})
}

// GetScriptState returns state keys for a script
func (h *Handler) GetScriptState(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	scriptID := uint(id)
	keys := h.engine.GetState().Keys(&scriptID)

	response := map[string]interface{}{
		"script_id": id,
		"keys":      keys,
		"count":     len(keys),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// DeleteScriptStateKey deletes a specific state key for a script
func (h *Handler) DeleteScriptStateKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, `{"error":"state key is required"}`, http.StatusBadRequest)
		return
	}

	scriptID := uint(id)
	if err := h.engine.GetState().Delete(&scriptID, key); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete state key: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "state key deleted successfully"})
}
