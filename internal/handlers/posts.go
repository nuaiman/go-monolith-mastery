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

// ============================================================
// PUBLIC READS
// ============================================================

// ListPostsHandler godoc
// @Summary      List posts
// @Tags         posts
// @Produce      json
// @Param        limit    query  int  false  "Page size (max 100)"
// @Param        offset   query  int  false  "Offset"
// @Param        user_id  query  string  false  "Filter by owner UUID"
// @Success      200  {array}  models.Post
// @Router       /posts [get]
func (h *Handler) ListPostsHandler(w http.ResponseWriter, r *http.Request) {
	limit, _ := utils.QueryInt(r, "limit")
	offset, _ := utils.QueryInt(r, "offset")
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	if userIDStr, ok := utils.QueryStr(r, "user_id"); ok {
		if !utils.IsValidUUID(userIDStr) {
			utils.ErrorJson(w, r, http.StatusBadRequest, "invalid user_id")
			return
		}
		posts, err := h.app.Models.Post.ListByUser(r.Context(), userIDStr, limit, offset)
		if err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch posts")
			return
		}
		utils.SuccessJson(w, r, http.StatusOK, "posts list", posts)
		return
	}

	posts, err := h.app.Models.Post.List(r.Context(), limit, offset)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch posts")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "posts list", posts)
}

// GetPostHandler godoc
// @Summary      Get one post
// @Tags         posts
// @Produce      json
// @Param        id  path  string  true  "Post UUID"
// @Success      200  {object}  models.Post
// @Failure      404  {object}  utils.Response
// @Router       /posts/{id} [get]
func (h *Handler) GetPostHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	post, err := h.app.Models.Post.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch post")
		return
	}
	if post == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "post not found")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "post details", post)
}

// ============================================================
// CREATE
// ============================================================

// CreatePostHandler godoc
// @Summary      Create a post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{title=string,body=string,image_url=string}  true  "Post"
// @Success      201   {object}  models.Post
// @Failure      401   {object}  utils.Response
// @Router       /posts [post]
func (h *Handler) CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}

	type createRequest struct {
		Title    string  `json:"title"`
		Body     string  `json:"body"`
		ImageURL *string `json:"image_url,omitempty"`
	}

	var req createRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == "" || req.Body == "" {
		utils.ErrorJson(w, r, http.StatusBadRequest, "title and body are required")
		return
	}
	if len(req.Title) > 256 {
		utils.ErrorJson(w, r, http.StatusBadRequest, "title must be 256 characters or less")
		return
	}

	post := &models.Post{
		UserID:   userID,
		Title:    req.Title,
		Body:     req.Body,
		ImageURL: req.ImageURL,
	}

	if err := h.app.Models.Post.Insert(r.Context(), post); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to create post")
		return
	}

	utils.SuccessJson(w, r, http.StatusCreated, "post created", post)
}

// ============================================================
// UPDATE
// ============================================================

// UpdatePostHandler godoc
// @Summary      Update a post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Post UUID"
// @Param        body  body  object{title=string,body=string,image_url=string}  true  "Fields"
// @Success      200   {object}  models.Post
// @Failure      403   {object}  utils.Response
// @Router       /posts/{id} [put]
func (h *Handler) UpdatePostHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}
	role, _ := middlewares.GetUserRoleFromContext(r)

	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	type updateRequest struct {
		Title    *string `json:"title,omitempty"`
		Body     *string `json:"body,omitempty"`
		ImageURL *string `json:"image_url,omitempty"`
	}

	var req updateRequest
	if err := utils.ReadJson(w, r, &req); err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	post, err := h.app.Models.Post.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch post")
		return
	}
	if post == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "post not found")
		return
	}

	isOwner := post.UserID == userID
	isAdmin := role == constants.RoleAdmin || role == constants.RoleSuperAdmin
	if !isOwner && !isAdmin {
		utils.ErrorJson(w, r, http.StatusForbidden, "you can only edit your own posts")
		return
	}

	if req.Title != nil {
		if *req.Title == "" {
			utils.ErrorJson(w, r, http.StatusBadRequest, "title cannot be empty")
			return
		}
		if len(*req.Title) > 256 {
			utils.ErrorJson(w, r, http.StatusBadRequest, "title must be 256 characters or less")
			return
		}
		post.Title = *req.Title
	}
	if req.Body != nil {
		if *req.Body == "" {
			utils.ErrorJson(w, r, http.StatusBadRequest, "body cannot be empty")
			return
		}
		post.Body = *req.Body
	}
	if req.ImageURL != nil {
		post.ImageURL = req.ImageURL
	}

	if isAdmin {
		if err := h.app.Models.Post.UpdateAny(r.Context(), post); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update post")
			return
		}
	} else {
		post.UserID = userID
		if err := h.app.Models.Post.Update(r.Context(), post); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update post")
			return
		}
	}

	utils.SuccessJson(w, r, http.StatusOK, "post updated", post)
}

// ============================================================
// DELETE
// ============================================================

// DeletePostHandler godoc
// @Summary      Delete a post
// @Tags         posts
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Post UUID"
// @Success      200  {object}  utils.Response
// @Failure      403  {object}  utils.Response
// @Router       /posts/{id} [delete]
func (h *Handler) DeletePostHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}
	role, _ := middlewares.GetUserRoleFromContext(r)

	id, ok := utils.ParamUUID(w, r)
	if !ok {
		return
	}

	post, err := h.app.Models.Post.GetByID(r.Context(), id)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to fetch post")
		return
	}
	if post == nil {
		utils.ErrorJson(w, r, http.StatusNotFound, "post not found")
		return
	}

	isOwner := post.UserID == userID
	isAdmin := role == constants.RoleAdmin || role == constants.RoleSuperAdmin
	if !isOwner && !isAdmin {
		utils.ErrorJson(w, r, http.StatusForbidden, "you can only delete your own posts")
		return
	}

	if isAdmin {
		if err := h.app.Models.Post.DeleteAny(r.Context(), id); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to delete post")
			return
		}
	} else {
		if err := h.app.Models.Post.Delete(r.Context(), id, userID); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to delete post")
			return
		}
	}

	postDir := filepath.Join(h.app.Config.BucketDir, "posts", id)
	if err := os.RemoveAll(postDir); err != nil {
		log.Printf("⚠️  failed to clean bucket for post %s: %v", id, err)
	}

	utils.SuccessJson(w, r, http.StatusOK, "post deleted", nil)
}
