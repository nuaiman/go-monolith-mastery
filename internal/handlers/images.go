package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"backend/internal/constants"
	"backend/internal/middlewares"
	"backend/internal/utils"
)

// ============================================================
// USER AVATAR
// ============================================================

// UploadUserImageHandler godoc
// @Summary      Upload user avatar
// @Tags         uploads
// @Accept       mpfd
// @Produce      json
// @Security     BearerAuth
// @Param        file  formData  file  true  "Image file (jpeg/png/webp/gif, 5MB)"
// @Success      201   {object}  map[string]any
// @Failure      400   {object}  utils.Response
// @Router       /uploads/user [post]
func (h *Handler) UploadUserImageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}

	file, header, uploadErr := readUploadedFile(w, r, h.app.Config.MaxUploadBytes)
	if uploadErr != nil {
		utils.ErrorJson(w, r, uploadErr.status, uploadErr.message)
		return
	}
	defer file.Close()

	scope := "users/" + userID
	urlPath, size, mime, err := saveImage(h.app.Config.BucketDir, scope, file, header.Filename)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if err := sweepDir(h.app.Config.BucketDir, scope, filepath.Base(urlPath)); err != nil {
		log.Printf("⚠️  failed to sweep %s: %v", scope, err)
	}

	if err := h.app.Models.User.SetImageURL(r.Context(), userID, &urlPath); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update user")
		return
	}

	utils.SuccessJson(w, r, http.StatusCreated, "file uploaded", map[string]any{
		"url":        urlPath,
		"size_bytes": size,
		"mime_type":  mime,
	})
}

// DeleteUserImageHandler godoc
// @Summary      Remove user avatar
// @Tags         uploads
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  utils.Response
// @Failure      401  {object}  utils.Response
// @Router       /uploads/user [delete]
func (h *Handler) DeleteUserImageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}

	scope := "users/" + userID
	if err := sweepDir(h.app.Config.BucketDir, scope, ""); err != nil {
		log.Printf("⚠️  failed to delete %s: %v", scope, err)
	}

	if err := h.app.Models.User.SetImageURL(r.Context(), userID, nil); err != nil {
		utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update user")
		return
	}

	utils.SuccessJson(w, r, http.StatusOK, "image removed", nil)
}

// ============================================================
// POST IMAGE
// ============================================================

// UploadPostImageHandler godoc
// @Summary      Upload post image
// @Tags         uploads
// @Accept       mpfd
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      string  true  "Post UUID"
// @Param        file  formData  file    true  "Image file (jpeg/png/webp/gif, 5MB)"
// @Success      201   {object}  map[string]any
// @Failure      403   {object}  utils.Response
// @Router       /uploads/post/{id} [post]
func (h *Handler) UploadPostImageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}
	role, _ := middlewares.GetUserRoleFromContext(r)

	postID, ok := utils.ParamUUID(w, r, "id")
	if !ok {
		return
	}

	post, err := h.app.Models.Post.GetByID(r.Context(), postID)
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
		utils.ErrorJson(w, r, http.StatusForbidden, "you can only upload to your own posts")
		return
	}

	file, header, uploadErr := readUploadedFile(w, r, h.app.Config.MaxUploadBytes)
	if uploadErr != nil {
		utils.ErrorJson(w, r, uploadErr.status, uploadErr.message)
		return
	}
	defer file.Close()

	scope := "posts/" + postID
	urlPath, size, mime, err := saveImage(h.app.Config.BucketDir, scope, file, header.Filename)
	if err != nil {
		utils.ErrorJson(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if err := sweepDir(h.app.Config.BucketDir, scope, filepath.Base(urlPath)); err != nil {
		log.Printf("⚠️  failed to sweep %s: %v", scope, err)
	}

	if isAdmin {
		if err := h.app.Models.Post.SetImageURLAny(r.Context(), postID, &urlPath); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update post")
			return
		}
	} else {
		if err := h.app.Models.Post.SetImageURL(r.Context(), postID, userID, &urlPath); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update post")
			return
		}
	}

	utils.SuccessJson(w, r, http.StatusCreated, "file uploaded", map[string]any{
		"url":        urlPath,
		"size_bytes": size,
		"mime_type":  mime,
	})
}

// DeletePostImageHandler godoc
// @Summary      Remove post image
// @Tags         uploads
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Post UUID"
// @Success      200  {object}  utils.Response
// @Failure      403  {object}  utils.Response
// @Router       /uploads/post/{id} [delete]
func (h *Handler) DeletePostImageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r)
	if !ok {
		utils.ErrorJson(w, r, http.StatusUnauthorized, "login required")
		return
	}
	role, _ := middlewares.GetUserRoleFromContext(r)

	postID, ok := utils.ParamUUID(w, r, "id")
	if !ok {
		return
	}

	post, err := h.app.Models.Post.GetByID(r.Context(), postID)
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
		utils.ErrorJson(w, r, http.StatusForbidden, "you can only modify your own posts")
		return
	}

	scope := "posts/" + postID
	if err := sweepDir(h.app.Config.BucketDir, scope, ""); err != nil {
		log.Printf("⚠️  failed to delete %s: %v", scope, err)
	}

	if isAdmin {
		if err := h.app.Models.Post.SetImageURLAny(r.Context(), postID, nil); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update post")
			return
		}
	} else {
		if err := h.app.Models.Post.SetImageURL(r.Context(), postID, userID, nil); err != nil {
			utils.ErrorJson(w, r, http.StatusInternalServerError, "failed to update post")
			return
		}
	}

	utils.SuccessJson(w, r, http.StatusOK, "image removed", nil)
}

// ============================================================
// HELPERS
// ============================================================

type uploadError struct {
	status  int
	message string
}

func readUploadedFile(w http.ResponseWriter, r *http.Request, maxBytes int64) (multipart.File, *multipart.FileHeader, *uploadError) {
	if r.ContentLength > maxBytes {
		return nil, nil, &uploadError{http.StatusRequestEntityTooLarge, "file too large"}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		return nil, nil, &uploadError{http.StatusRequestEntityTooLarge, "file too large or malformed form"}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile {
			return nil, nil, &uploadError{http.StatusBadRequest, "form field 'file' is required"}
		}
		return nil, nil, &uploadError{http.StatusBadRequest, "invalid file upload"}
	}

	return file, header, nil
}

func sweepDir(bucketRoot, scope, keepFilename string) error {
	absDir := filepath.Join(bucketRoot, scope)

	if keepFilename == "" {
		return os.RemoveAll(absDir)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.Name() == keepFilename {
			continue
		}
		if err := os.RemoveAll(filepath.Join(absDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// SLUG + MIME
// ============================================================

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)
	multiDash    = regexp.MustCompile(`-+`)
)

const maxSlugLength = 64

var allowedMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func slugify(name string) string {
	s := strings.ToLower(name)
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > maxSlugLength {
		s = s[:maxSlugLength]
		s = strings.Trim(s, "-")
	}
	if s == "" {
		s = randomHex()
	}
	return s
}

func randomHex() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ============================================================
// SAVE
// ============================================================

var (
	dirMu    sync.Mutex
	dirLocks = map[string]*sync.Mutex{}
)

func dirLock(dir string) *sync.Mutex {
	dirMu.Lock()
	defer dirMu.Unlock()
	if m, ok := dirLocks[dir]; ok {
		return m
	}
	m := &sync.Mutex{}
	dirLocks[dir] = m
	return m
}

func saveImage(bucketRoot, scope string, file multipart.File, originalName string) (urlPath string, size int64, mime string, err error) {
	header := make([]byte, 512)
	n, readErr := file.Read(header)
	if readErr != nil && readErr != io.EOF {
		return "", 0, "", fmt.Errorf("read file header: %w", readErr)
	}
	header = header[:n]

	mime = http.DetectContentType(header)
	ext, ok := allowedMIME[mime]
	if !ok {
		return "", 0, "", fmt.Errorf("unsupported image type: %s", mime)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", 0, "", fmt.Errorf("seek file: %w", err)
	}

	absDir := filepath.Join(bucketRoot, scope)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", 0, "", fmt.Errorf("create dir: %w", err)
	}

	base := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	slug := slugify(base)

	lock := dirLock(absDir)
	lock.Lock()
	defer lock.Unlock()

	filename, err := uniqueFilename(absDir, slug, ext)
	if err != nil {
		return "", 0, "", err
	}

	absPath := filepath.Join(absDir, filename)
	dst, err := os.Create(absPath)
	if err != nil {
		return "", 0, "", fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		_ = os.Remove(absPath)
		return "", 0, "", fmt.Errorf("write file: %w", err)
	}

	urlPath = "/bucket/" + filepath.ToSlash(filepath.Join(scope, filename))
	return urlPath, written, mime, nil
}

func uniqueFilename(dir, slug, ext string) (string, error) {
	candidate := slug + ext
	if !fileExists(filepath.Join(dir, candidate)) {
		return candidate, nil
	}
	for i := 1; i < 1000; i++ {
		candidate = fmt.Sprintf("%s-%d%s", slug, i, ext)
		if !fileExists(filepath.Join(dir, candidate)) {
			return candidate, nil
		}
	}
	return randomHex() + ext, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
