package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github/bromq-dev/bromq/internal/badgerstore"
	"github/bromq-dev/bromq/internal/storage"

	"gorm.io/datatypes"
)

// === Script Management Handlers ===

// ListScripts godoc
// @Summary List scripts
// @Description Get paginated list of JavaScript scripts with their triggers
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param search query string false "Search by script name"
// @Param sortBy query string false "Sort field" default(id)
// @Param sortOrder query string false "Sort order (asc/desc)" default(asc)
// @Success 200 {object} PaginatedResponse{data=[]storage.Script}
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /scripts [get]
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

// GetScript godoc
// @Summary Get script
// @Description Get a single JavaScript script by ID with its triggers
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Success 200 {object} storage.Script
// @Failure 400 {object} ErrorResponse "Invalid script ID"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse "Script not found"
// @Router /scripts/{id} [get]
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

// CreateScript godoc
// @Summary Create script
// @Description Create a new JavaScript script with triggers for MQTT events (publish, connect, disconnect, subscribe)
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param script body CreateScriptRequest true "Script content and triggers"
// @Success 201 {object} storage.Script
// @Failure 400 {object} ErrorResponse "Invalid request or validation error"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /scripts [post]
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
	if req.Content == "" {
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
			Type:     t.Type,
			Topic:    t.Topic,
			Priority: t.Priority,
			Enabled:  t.Enabled,
		}
	}

	script, err := h.db.CreateScript(req.Name, req.Description, req.Content, req.Enabled, metadata, triggers)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create script: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(script)
}

// UpdateScript godoc
// @Summary Update script
// @Description Update an existing JavaScript script and its triggers
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Param script body UpdateScriptRequest true "Updated script content and triggers"
// @Success 200 {object} storage.Script
// @Failure 400 {object} ErrorResponse "Invalid script ID or validation error"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse "Script not found"
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be modified"
// @Failure 500 {object} ErrorResponse
// @Router /scripts/{id} [put]
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
			Type:     t.Type,
			Topic:    t.Topic,
			Priority: t.Priority,
			Enabled:  t.Enabled,
		}
	}

	if err := h.db.UpdateScript(uint(id), req.Name, req.Description, req.Content, req.Enabled, metadata, triggers); err != nil {
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

// DeleteScript godoc
// @Summary Delete script
// @Description Delete a JavaScript script and all its triggers
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Invalid script ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 404 {object} ErrorResponse "Script not found"
// @Failure 409 {object} ErrorResponse "Provisioned resource cannot be deleted"
// @Failure 500 {object} ErrorResponse
// @Router /scripts/{id} [delete]
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

// EnableScript godoc
// @Summary Enable/disable script
// @Description Toggle script enabled status to control whether it executes on MQTT events
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Param enabled body object{enabled=bool} true "Enable/disable flag"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Invalid script ID or request"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /scripts/{id}/enable [put]
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

// TestScript godoc
// @Summary Test script
// @Description Test a JavaScript script with mock event data without saving it to the database
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param test body TestScriptRequest true "Script content and mock event data"
// @Success 200 {object} object{success=bool,execution_time_ms=number,logs=[]string,error=string}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse
// @Router /scripts/test [post]
func (h *Handler) TestScript(w http.ResponseWriter, r *http.Request) {
	var req TestScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Content == "" {
		http.Error(w, `{"error":"script content is required"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, `{"error":"trigger type is required"}`, http.StatusBadRequest)
		return
	}

	// Test the script
	result := h.engine.TestScript(req.Content, req.Type, req.EventData)

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

// GetScriptLogs godoc
// @Summary Get script logs
// @Description Get paginated execution logs for a specific script with optional level filtering
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param level query string false "Filter by log level (debug, info, warn, error)"
// @Success 200 {object} PaginatedResponse{data=[]badgerstore.ScriptLogEntry}
// @Failure 400 {object} ErrorResponse "Invalid script ID"
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /scripts/{id}/logs [get]
func (h *Handler) GetScriptLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	params := parsePaginationParams(r)
	level := r.URL.Query().Get("level") // Optional filter by level

	badger := h.engine.GetBadger()
	logs, total, err := badger.ListScriptLogs(uint(id), params.Page, params.PageSize, level)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list logs: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if logs == nil {
		logs = []badgerstore.ScriptLogEntry{}
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

// ClearScriptLogs godoc
// @Summary Clear script logs
// @Description Delete all execution logs for a specific script
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Invalid script ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /scripts/{id}/logs [delete]
func (h *Handler) ClearScriptLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid script ID"}`, http.StatusBadRequest)
		return
	}

	badger := h.engine.GetBadger()
	if err := badger.ClearScriptLogs(uint(id)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to clear logs: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "logs cleared successfully"})
}

// GetScriptState godoc
// @Summary Get script state
// @Description Get all persistent state keys stored by a script
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Success 200 {object} object{script_id=int,keys=[]string,count=int}
// @Failure 400 {object} ErrorResponse "Invalid script ID"
// @Failure 401 {object} ErrorResponse
// @Router /scripts/{id}/state [get]
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

// DeleteScriptStateKey godoc
// @Summary Delete script state key
// @Description Delete a specific persistent state key for a script
// @Tags Scripts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Script ID"
// @Param key path string true "State key to delete"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Invalid script ID or missing key"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /scripts/{id}/state/{key} [delete]
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
