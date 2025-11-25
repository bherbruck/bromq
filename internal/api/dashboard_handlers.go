package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github/bromq-dev/bromq/internal/storage"
)

// === Admin User Management Handlers ===

// ListDashboardUsers godoc
// @Summary List dashboard users
// @Description Get paginated list of dashboard admin users
// @Tags Dashboard Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(25)
// @Param search query string false "Search by username"
// @Param sortBy query string false "Sort field" default(id)
// @Param sortOrder query string false "Sort order (asc/desc)" default(asc)
// @Success 200 {object} PaginatedResponse{data=[]storage.DashboardUser}
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /dashboard/users [get]
func (h *Handler) ListDashboardUsers(w http.ResponseWriter, r *http.Request) {
	params := parsePaginationParams(r)

	users, total, err := h.db.ListDashboardUsersPaginated(params.Page, params.PageSize, params.Search, params.SortBy, params.SortOrder)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list admin users: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if users == nil {
		users = []storage.DashboardUser{}
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

// CreateDashboardUser godoc
// @Summary Create dashboard user
// @Description Create a new dashboard admin user
// @Tags Dashboard Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body CreateDashboardUserRequest true "User details"
// @Success 201 {object} storage.DashboardUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /dashboard/users [post]
func (h *Handler) CreateDashboardUser(w http.ResponseWriter, r *http.Request) {
	var req CreateDashboardUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	user, err := h.db.CreateDashboardUser(req.Username, req.Password, req.Role)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create admin user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// GetDashboardUser godoc
// @Summary Get dashboard user
// @Description Get a single dashboard user by ID
// @Tags Dashboard Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} storage.DashboardUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /dashboard/users/{id} [get]
func (h *Handler) GetDashboardUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	user, err := h.db.GetDashboardUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"admin user not found: %s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// UpdateDashboardUser godoc
// @Summary Update dashboard user
// @Description Update dashboard user information (username, role)
// @Tags Dashboard Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param user body UpdateDashboardUserRequest true "Updated user details"
// @Success 200 {object} storage.DashboardUser
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /dashboard/users/{id} [put]
func (h *Handler) UpdateDashboardUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	var req UpdateDashboardUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateDashboardUser(id, req.Username, req.Role); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update admin user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	user, err := h.db.GetDashboardUser(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to get admin user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// DeleteDashboardUser godoc
// @Summary Delete dashboard user
// @Description Delete a dashboard user (cannot delete yourself)
// @Tags Dashboard Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Invalid ID or attempting to delete yourself"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /dashboard/users/{id} [delete]
func (h *Handler) DeleteDashboardUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	// Prevent deleting yourself
	claims, ok := GetUserFromContext(r)
	if ok && claims.UserID == id {
		http.Error(w, `{"error":"cannot delete your own account"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteDashboardUser(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete admin user: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "admin user deleted"})
}

// UpdateDashboardUserPassword godoc
// @Summary Update user password (admin)
// @Description Admin endpoint to update any dashboard user's password
// @Tags Dashboard Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param password body UpdateAdminPasswordRequest true "New password"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse "Admin only"
// @Failure 500 {object} ErrorResponse
// @Router /dashboard/users/{id}/password [put]
func (h *Handler) UpdateDashboardUserPassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idVal, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}
	id := uint(idVal)

	var req UpdateAdminPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, `{"error":"password cannot be empty"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateDashboardUserPassword(id, req.Password); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update password: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "password updated"})
}

// ChangePassword godoc
// @Summary Change own password
// @Description Authenticated users can change their own password
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param passwords body ChangePasswordRequest true "Current and new password"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Invalid current password"
// @Failure 500 {object} ErrorResponse
// @Router /auth/change-password [put]
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context (set by auth middleware)
	claims, ok := GetUserFromContext(r)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid request: %s"}`, err), http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, `{"error":"current_password and new_password are required"}`, http.StatusBadRequest)
		return
	}

	// Verify current password
	user, err := h.db.AuthenticateDashboardUser(claims.Username, req.CurrentPassword)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"authentication failed: %s"}`, err), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, `{"error":"current password is incorrect"}`, http.StatusUnauthorized)
		return
	}

	// Update to new password
	if err := h.db.UpdateDashboardUserPassword(claims.UserID, req.NewPassword); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to update password: %s"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SuccessResponse{Message: "password changed successfully"})
}
