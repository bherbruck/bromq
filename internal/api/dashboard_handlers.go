package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github/bherbruck/mqtt-server/internal/storage"
)

// === Admin User Management Handlers ===

// ListDashboardUsers returns all admin users
func (h *Handler) ListDashboardUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListDashboardUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list admin users: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if users == nil {
		users = []storage.DashboardUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// CreateDashboardUser creates a new admin user
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
	json.NewEncoder(w).Encode(user)
}

// UpdateDashboardUser updates an admin user's information
func (h *Handler) UpdateDashboardUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(user)
}

// DeleteDashboardUser deletes an admin user
func (h *Handler) DeleteDashboardUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(SuccessResponse{Message: "admin user deleted"})
}

// UpdateDashboardUserPassword updates an admin user's password
func (h *Handler) UpdateDashboardUserPassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(SuccessResponse{Message: "password updated"})
}

// ChangePassword allows authenticated admin users to change their own password
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
	json.NewEncoder(w).Encode(SuccessResponse{Message: "password changed successfully"})
}
