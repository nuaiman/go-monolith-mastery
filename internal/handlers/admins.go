package handlers

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"backend/internal/constants"
	"backend/internal/middlewares"
	"backend/internal/models"
	"backend/internal/utils"
)

// sanitizeUser clears secrets before sending to the client.
func sanitizeUser(u *models.User) *models.User {
	u.Password = nil
	u.RefreshToken = nil
	u.RefreshTokenAt = nil
	return u
}

// ============================================================
// ADMIN TIER
// ============================================================

// AdminListUsersHandler godoc
// @Summary      List all users
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        role  query  string  false  "Filter by role (user/admin/superadmin)"
// @Success      200   {array}  models.User
// @Failure      403   {object}  utils.Response
// @Router       /admin/users [get]
func (h *Handler) AdminListUsersHandler(w http.ResponseWriter, r *http.Request) {
	var users []*models.User
	var err error

	if role, ok := utils.QueryStr(r, "role"); ok {
		if !constants.IsValidRole(role) {
			utils.ErrorJson(w, r, http.StatusBadRequest, "invalid role filter")
			return
		}
		users, err = h.app.Models.User.GetByRole(r.Context(), role)
	} else {
		users, err = h.app.Models.User.GetAll(r.Context())
	}

	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	for _, u := range users {
		sanitizeUser(u)
	}

	utils.SuccessJson(w, r, http.StatusOK, "users list", users)
}

// AdminGetUserHandler godoc
// @Summary      Get one user
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User UUID"
// @Success      200  {object}  models.User
// @Failure      404  {object}  utils.Response
// @Router       /admin/users/{id} [get]
func (h *Handler) AdminGetUserHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	user, err := h.app.Models.User.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if user == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "user details", sanitizeUser(user))
}

// AdminUpdateUserHandler godoc
// @Summary      Update a user
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "User UUID"
// @Param        body  body  object{email=string,name=string,image_url=string,is_active=bool}  true  "Fields"
// @Success      200   {object}  models.User
// @Failure      403   {object}  utils.Response
// @Router       /admin/users/{id} [put]
func (h *Handler) AdminUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}
	callerRole, _ := middlewares.GetUserRoleFromContext(r)

	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	type updateRequest struct {
		Email    *string `json:"email,omitempty"`
		Name     *string `json:"name,omitempty"`
		ImageURL *string `json:"image_url,omitempty"`
		IsActive *bool   `json:"is_active,omitempty"`
	}

	var req updateRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	target, err := h.app.Models.User.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if target == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	if !constants.CanModerate(callerRole, target.Role) {
		utils.ErrorJson(w, r, http.StatusForbidden, "cannot modify this user")
		return
	}

	if req.IsActive != nil && *req.IsActive == false && target.ID == callerID {
		utils.ErrorJson(w, r, http.StatusBadRequest, "cannot deactivate your own account")
		return
	}

	if req.Email != nil {
		if *req.Email == "" {
			utils.ErrorJson(w, r, http.StatusBadRequest, "email cannot be empty")
			return
		}
		existing, err := h.app.Models.User.GetByEmail(r.Context(), *req.Email)
		if err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to check email")
			return
		}
		if existing != nil && existing.ID != target.ID {
			utils.ErrorJson(w, r, http.StatusConflict, "email already in use")
			return
		}
		target.Email = *req.Email
	}
	if req.Name != nil {
		target.Name = req.Name
	}
	if req.ImageURL != nil {
		target.ImageURL = req.ImageURL
	}
	if req.IsActive != nil {
		target.IsActive = *req.IsActive
	}

	if err := h.app.Models.User.UpdateByAdmin(r.Context(), target); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update user")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "user updated", sanitizeUser(target))
}

// AdminSetUserActiveHandler godoc
// @Summary      Activate or deactivate a user
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "User UUID"
// @Param        body  body  object{is_active=bool}  true  "Active flag"
// @Success      200   {object}  utils.Response
// @Failure      403   {object}  utils.Response
// @Router       /admin/users/{id}/active [put]
func (h *Handler) AdminSetUserActiveHandler(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}
	callerRole, _ := middlewares.GetUserRoleFromContext(r)

	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	type activeRequest struct {
		IsActive *bool `json:"is_active"`
	}

	var req activeRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IsActive == nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "is_active is required")
		return
	}

	target, err := h.app.Models.User.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if target == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	if !constants.CanModerate(callerRole, target.Role) {
		utils.ErrorJson(w, r, http.StatusForbidden, "cannot modify this user")
		return
	}
	if target.ID == callerID {
		utils.ErrorJson(w, r, http.StatusBadRequest, "cannot change your own active status")
		return
	}

	if err := h.app.Models.User.SetActive(r.Context(), target.ID, *req.IsActive); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update user")
		return
	}

	status := "activated"
	if !*req.IsActive {
		status = "deactivated"
	}

	utils.SuccessJson(w, r, http.StatusOK, "user "+status, nil)
}

// AdminDeleteUserHandler deletes a user and cleans up all their files:
// - bucket/users/<id>/         (their avatar)
// - bucket/posts/<post_id>/    (their posts' images, one dir per post)
//
// Post bucket directories must be enumerated BEFORE the DB delete,
// because ON DELETE CASCADE removes the post rows and we lose the IDs.
//
// @Summary      Delete a user
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User UUID"
// @Success      200  {object}  utils.Response
// @Failure      403  {object}  utils.Response
// @Router       /admin/users/{id} [delete]
func (h *Handler) AdminDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}
	callerRole, _ := middlewares.GetUserRoleFromContext(r)

	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	if id == callerID {
		utils.ErrorJson(w, r, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	target, err := h.app.Models.User.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if target == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	if !constants.CanModerate(callerRole, target.Role) {
		utils.ErrorJson(w, r, http.StatusForbidden, "cannot delete this user")
		return
	}

	// 1. Enumerate post IDs BEFORE the cascade runs.
	posts, err := h.app.Models.Post.ListByUser(r.Context(), id, 10000, 0)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to list user's posts")
		return
	}
	postIDs := make([]string, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}

	// 2. Delete the user row (cascade removes their posts).
	if err := h.app.Models.User.Delete(r.Context(), id); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to delete user")
		return
	}

	// 3. Best-effort filesystem cleanup.
	userDir := filepath.Join(h.app.Config.BucketDir, "users", id)
	if err := os.RemoveAll(userDir); err != nil {
		log.Printf("⚠️  failed to clean bucket for user %s: %v", id, err)
	}
	for _, pid := range postIDs {
		postDir := filepath.Join(h.app.Config.BucketDir, "posts", pid)
		if err := os.RemoveAll(postDir); err != nil {
			log.Printf("⚠️  failed to clean bucket for post %s: %v", pid, err)
		}
	}

	utils.SuccessJson(w, r, http.StatusOK, "user deleted", nil)
}

// ============================================================
// SUPER TIER
// ============================================================

// SuperCreateAdminHandler godoc
// @Summary      Create a new admin
// @Tags         super
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{email=string,password=string,name=string}  true  "Admin"
// @Success      201   {object}  models.User
// @Failure      403   {object}  utils.Response
// @Router       /admin/users [post]
func (h *Handler) SuperCreateAdminHandler(w http.ResponseWriter, r *http.Request) {
	type createRequest struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Name     *string `json:"name,omitempty"`
	}

	var req createRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		utils.ErrorJson(w, r, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(req.Password) < 8 {
		utils.ErrorJson(w, r, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	taken, err := h.app.Models.User.ExistsByEmail(r.Context(), req.Email)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to check email")
		return
	}
	if taken {
		utils.ErrorJson(w, r, http.StatusConflict, "email already registered")
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &models.User{
		Role:     constants.RoleAdmin,
		IsActive: true,
		Email:    req.Email,
		Password: &hashedPassword,
		Name:     req.Name,
	}

	if err := h.app.Models.User.Insert(r.Context(), user); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to create admin")
		return
	}

	utils.SuccessJson(w, r, http.StatusCreated, "admin created", sanitizeUser(user))
}

// SuperSetUserRoleHandler godoc
// @Summary      Change a user's role
// @Tags         super
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "User UUID"
// @Param        body  body  object{role=string}  true  "Role (user/admin)"
// @Success      200   {object}  utils.Response
// @Failure      403   {object}  utils.Response
// @Router       /admin/users/{id}/role [put]
func (h *Handler) SuperSetUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	type roleRequest struct {
		Role string `json:"role"`
	}

	var req roleRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != constants.RoleUser && req.Role != constants.RoleAdmin {
		utils.ErrorJson(w, r, http.StatusBadRequest, "role must be 'user' or 'admin'")
		return
	}

	if id == callerID {
		utils.ErrorJson(w, r, http.StatusBadRequest, "cannot change your own role")
		return
	}

	target, err := h.app.Models.User.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if target == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	if target.Role == constants.RoleSuperAdmin {
		utils.ErrorJson(w, r, http.StatusForbidden, "cannot change another superadmin's role")
		return
	}

	if err := h.app.Models.User.SetRole(r.Context(), id, req.Role); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update role")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "role updated to "+req.Role, nil)
}
