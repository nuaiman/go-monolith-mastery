package handlers

import (
	"net/http"
	"time"

	"backend/internal/constants"
	"backend/internal/middlewares"
	"backend/internal/models"
	"backend/internal/utils"
)

// ============================================================
// AUTH
// ============================================================

// LoginHandler godoc
// @Summary      Login with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object{email=string,password=string}  true  "Credentials"
// @Success      200   {object}  map[string]any
// @Failure      401   {object}  utils.Response
// @Router       /auth/login [post]
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	type loginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req loginRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		utils.ErrorJson(w, r, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.app.Models.User.GetByEmail(r.Context(), req.Email)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if user == nil || user.Password == nil {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !utils.ComparePassword(*user.Password, req.Password) {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.IsActive {
		utils.ErrorJson(w, r, http.StatusForbidden, "account is not active")
		return
	}

	refreshToken, err := utils.GenerateRefreshToken()
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to generate session")
		return
	}
	if err := h.app.Models.User.UpdateRefreshToken(r.Context(), user.ID, &refreshToken); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to create session")
		return
	}

	accessToken, err := utils.GenerateAccessToken(
		user.ID, user.Role,
		h.app.Config.JWTKey,
		h.app.Config.JWTExpiryHours,
	)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   h.app.Config.Env == "production",
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   int(h.app.Config.RefreshTokenAge.Seconds()),
	})

	user.Password = nil
	user.RefreshToken = nil
	user.RefreshTokenAt = nil

	utils.SuccessJson(w, r, http.StatusOK, "login successful", map[string]any{
		"access_token": accessToken,
		"expires_at":   time.Now().Add(time.Duration(h.app.Config.JWTExpiryHours) * time.Hour).Unix(),
		"expires_in":   h.app.Config.JWTExpiryHours * 3600,
		"user":         user,
	})
}

// RefreshHandler godoc
// @Summary      Refresh access token
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      401  {object}  utils.Response
// @Router       /auth/refresh [post]
func (h *Handler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "missing session")
		return
	}

	user, err := h.app.Models.User.GetByRefreshToken(r.Context(), cookie.Value)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to validate session")
		return
	}
	if user == nil {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "invalid session")
		return
	}
	if !user.IsActive {
		utils.ErrorJson(w, r, http.StatusForbidden, "account is not active")
		return
	}

	newRefreshToken, err := utils.GenerateRefreshToken()
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to generate session")
		return
	}
	if err := h.app.Models.User.UpdateRefreshToken(r.Context(), user.ID, &newRefreshToken); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update session")
		return
	}

	accessToken, err := utils.GenerateAccessToken(
		user.ID, user.Role,
		h.app.Config.JWTKey,
		h.app.Config.JWTExpiryHours,
	)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to generate access token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		HttpOnly: true,
		Secure:   h.app.Config.Env == "production",
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   int(h.app.Config.RefreshTokenAge.Seconds()),
	})

	utils.SuccessJson(w, r, http.StatusOK, "session refreshed", map[string]any{
		"access_token": accessToken,
		"expires_at":   time.Now().Add(time.Duration(h.app.Config.JWTExpiryHours) * time.Hour).Unix(),
		"expires_in":   h.app.Config.JWTExpiryHours * 3600,
	})
}

// LogoutHandler godoc
// @Summary      Logout current user
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.Response
// @Failure      401  {object}  utils.Response
// @Router       /auth/logout [post]
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.app.Models.User.UpdateRefreshToken(r.Context(), userID, nil); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to logout")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.app.Config.Env == "production",
	})

	utils.SuccessJson(w, r, http.StatusOK, "logged out successfully", nil)
}

// ============================================================
// PROFILE
// ============================================================

// GetCurrentUserHandler godoc
// @Summary      Get current user details
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  models.User
// @Failure      401  {object}  utils.Response
// @Router       /auth/me [get]
func (h *Handler) GetCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.app.Models.User.GetByID(r.Context(), userID)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if user == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	user.Password = nil
	user.RefreshToken = nil
	user.RefreshTokenAt = nil

	utils.SuccessJson(w, r, http.StatusOK, "user details", user)
}

// UpdateProfileHandler godoc
// @Summary      Update own profile
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{name=string,image_url=string}  true  "Profile fields"
// @Success      200   {object}  models.User
// @Failure      401   {object}  utils.Response
// @Router       /auth/profile [put]
func (h *Handler) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	type updateProfileRequest struct {
		Name     *string `json:"name,omitempty"`
		ImageURL *string `json:"image_url,omitempty"`
	}

	var req updateProfileRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.app.Models.User.GetByID(r.Context(), userID)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if user == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}

	if req.Name != nil {
		user.Name = req.Name
	}
	if req.ImageURL != nil {
		user.ImageURL = req.ImageURL
	}

	if err := h.app.Models.User.UpdateProfile(r.Context(), user); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update profile")
		return
	}

	user.Password = nil
	user.RefreshToken = nil
	user.RefreshTokenAt = nil

	utils.SuccessJson(w, r, http.StatusOK, "profile updated", user)
}

// ChangePasswordHandler godoc
// @Summary      Change own password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{current_password=string,new_password=string}  true  "Passwords"
// @Success      200   {object}  utils.Response
// @Failure      401   {object}  utils.Response
// @Router       /auth/change-password [put]
func (h *Handler) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	type changePasswordRequest struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	var req changePasswordRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		utils.ErrorJson(w, r, http.StatusBadRequest, "current and new password are required")
		return
	}
	if len(req.NewPassword) < 8 {
		utils.ErrorJson(w, r, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	user, err := h.app.Models.User.GetByID(r.Context(), userID)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	if user == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "user not found")
		return
	}
	if user.Password == nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "no password set for this account")
		return
	}
	if !utils.ComparePassword(*user.Password, req.CurrentPassword) {
		utils.ErrorJson(w, r, http.StatusBadRequest, "current password is incorrect")
		return
	}

	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to process password")
		return
	}
	if err := h.app.Models.User.UpdatePassword(r.Context(), userID, hashedPassword); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update password")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "password changed successfully", nil)
}

// ============================================================
// PUBLIC SIGNUP
// ============================================================

// RegisterHandler godoc
// @Summary      Register a new user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object{email=string,password=string,name=string}  true  "Registration"
// @Success      201   {object}  models.User
// @Failure      409   {object}  utils.Response
// @Router       /auth/register [post]
func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	type registerRequest struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Name     *string `json:"name,omitempty"`
	}

	var req registerRequest
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
		Role:     constants.RoleUser,
		IsActive: true,
		Email:    req.Email,
		Password: &hashedPassword,
		Name:     req.Name,
	}

	if err := h.app.Models.User.Insert(r.Context(), user); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}

	user.Password = nil
	user.RefreshToken = nil
	user.RefreshTokenAt = nil

	utils.SuccessJson(w, r, http.StatusCreated, "user registered successfully", user)
}

// ============================================================
// PUBLIC USER READS
// ============================================================

// GetUserHandler godoc
// @Summary      Get a user by ID
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "User UUID"
// @Success      200  {object}  models.User
// @Failure      401  {object}  utils.Response
// @Failure      404  {object}  utils.Response
// @Router       /users/{id} [get]
func (h *Handler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
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

	user.Password = nil
	user.RefreshToken = nil
	user.RefreshTokenAt = nil

	utils.SuccessJson(w, r, http.StatusOK, "user details", user)
}
