package router

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "backend/docs" // registers the generated swagger spec

	"backend/internal/app"
	"backend/internal/constants"
	"backend/internal/handlers"
	"backend/internal/middlewares"
	"backend/internal/utils"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func RegisterRouter(app *app.Application, handler *handlers.Handler) http.Handler {
	// ---------- Rate limiters (tiered by risk class) ----------
	//
	// All limiters are per-IP. If a client supplies X-Forwarded-For
	// and TRUST_PROXY=true, the first IP in that header is used.
	//
	//	Tier      Limit      Applies to
	//	──────    ────────   ─────────────────────────────────────────────
	//	strict    5/min      login, register, change-password
	//	write     60/hour    create/update/delete posts, update profile
	//	upload    30/hour    image uploads (POST/DELETE)
	//	read      300/hour   public GET endpoints
	//
	// Admin endpoints and /auth/me rely on the global limiter only
	// (RATE_LIMIT in config.env).
	strictLimiter := middlewares.NewRateLimiter(5, 1*time.Minute, app.Config.TrustProxy)
	writeLimiter := middlewares.NewRateLimiter(60, 1*time.Hour, app.Config.TrustProxy)
	uploadLimiter := middlewares.NewRateLimiter(30, 1*time.Hour, app.Config.TrustProxy)
	readLimiter := middlewares.NewRateLimiter(300, 1*time.Hour, app.Config.TrustProxy)

	// ---------- Auth helpers ----------
	protected := middlewares.Authenticate(app.Config.JWTKey)

	adminOrSuper := func(h http.HandlerFunc) http.HandlerFunc {
		return protected(middlewares.RequireRole(constants.RoleAdmin, constants.RoleSuperAdmin)(h))
	}
	superOnly := func(h http.HandlerFunc) http.HandlerFunc {
		return protected(middlewares.RequireRole(constants.RoleSuperAdmin)(h))
	}

	// ---------- Mux ----------
	mux := http.NewServeMux()

	// ============================================================
	// Static / docs (public, no rate limit beyond global)
	// ============================================================

	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	mux.Handle("/bucket/", http.StripPrefix("/bucket/",
		http.FileServer(http.Dir(app.Config.BucketDir)),
	))

	// ============================================================
	// Health (public)
	// ============================================================

	mux.HandleFunc("GET /api/healthz", handler.HealthzHandler)

	// ============================================================
	// Auth
	// ============================================================

	// Strict tier — brute-force defense
	mux.Handle("POST /api/auth/login",
		strictLimiter.LimitRate(http.HandlerFunc(handler.LoginHandler)))
	mux.Handle("POST /api/auth/register",
		strictLimiter.LimitRate(http.HandlerFunc(handler.RegisterHandler)))
	mux.Handle("PUT /api/auth/change-password",
		strictLimiter.LimitRate(http.HandlerFunc(protected(handler.ChangePasswordHandler))))

	// Unrestricted (safe by nature)
	mux.HandleFunc("POST /api/auth/refresh", handler.RefreshHandler)
	mux.HandleFunc("POST /api/auth/logout", protected(handler.LogoutHandler))
	mux.HandleFunc("GET /api/auth/me", protected(handler.GetCurrentUserHandler))

	// Write tier
	mux.Handle("PUT /api/auth/profile",
		writeLimiter.LimitRate(http.HandlerFunc(protected(handler.UpdateProfileHandler))))

	// ============================================================
	// Users
	// ============================================================

	mux.Handle("GET /api/users/{id}",
		readLimiter.LimitRate(http.HandlerFunc(protected(handler.GetUserHandler))))

	// ============================================================
	// Posts
	// ============================================================

	mux.Handle("GET /api/posts",
		readLimiter.LimitRate(http.HandlerFunc(handler.ListPostsHandler)))
	mux.Handle("GET /api/posts/{id}",
		readLimiter.LimitRate(http.HandlerFunc(handler.GetPostHandler)))
	mux.Handle("POST /api/posts",
		writeLimiter.LimitRate(http.HandlerFunc(protected(handler.CreatePostHandler))))
	mux.Handle("PUT /api/posts/{id}",
		writeLimiter.LimitRate(http.HandlerFunc(protected(handler.UpdatePostHandler))))
	mux.Handle("DELETE /api/posts/{id}",
		writeLimiter.LimitRate(http.HandlerFunc(protected(handler.DeletePostHandler))))

	// ============================================================
	// Uploads
	// ============================================================

	mux.Handle("POST /api/uploads/user",
		uploadLimiter.LimitRate(http.HandlerFunc(protected(handler.UploadUserImageHandler))))
	mux.Handle("POST /api/uploads/post/{id}",
		uploadLimiter.LimitRate(http.HandlerFunc(protected(handler.UploadPostImageHandler))))
	mux.Handle("DELETE /api/uploads/user",
		uploadLimiter.LimitRate(http.HandlerFunc(protected(handler.DeleteUserImageHandler))))
	mux.Handle("DELETE /api/uploads/post/{id}",
		uploadLimiter.LimitRate(http.HandlerFunc(protected(handler.DeletePostImageHandler))))

	// ============================================================
	// Admin (global limiter only)
	// ============================================================

	mux.HandleFunc("GET /api/admin/users", adminOrSuper(handler.AdminListUsersHandler))
	mux.HandleFunc("GET /api/admin/users/{id}", adminOrSuper(handler.AdminGetUserHandler))
	mux.HandleFunc("PUT /api/admin/users/{id}", adminOrSuper(handler.AdminUpdateUserHandler))
	mux.HandleFunc("PUT /api/admin/users/{id}/active", adminOrSuper(handler.AdminSetUserActiveHandler))
	mux.HandleFunc("DELETE /api/admin/users/{id}", adminOrSuper(handler.AdminDeleteUserHandler))

	// ============================================================
	// Super tier
	// ============================================================

	mux.HandleFunc("POST /api/admin/users", superOnly(handler.SuperCreateAdminHandler))
	mux.HandleFunc("PUT /api/admin/users/{id}/role", superOnly(handler.SuperSetUserRoleHandler))

	// ============================================================
	// SPA fallback (catch-all, must be last)
	// ============================================================

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") ||
			strings.HasPrefix(r.URL.Path, "/bucket") ||
			strings.HasPrefix(r.URL.Path, "/swagger") {
			utils.ErrorJson(w, r, http.StatusNotFound, "not found")
			return
		}

		path := filepath.Join(app.Config.PublicDir, r.URL.Path)
		if _, err := os.Stat(path); err == nil {
			http.ServeFile(w, r, path)
			return
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			indexPath := filepath.Join(path, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(app.Config.PublicDir, "index.html"))
	})

	// ============================================================
	// Global middleware chain
	// Order (outermost first):
	//   SecurityHeaders → RequestID → RequestLogger → Recoverer → CORS → mux
	// ============================================================

	withCORS := corsMiddleware(app)(mux)
	withRecover := recovererMiddleware(withCORS)
	withLogger := middlewares.RequestLogger(withRecover)
	withRequestID := middlewares.RequestID(withLogger)
	withSecurity := middlewares.SecurityHeaders(withRequestID)

	return withSecurity
}

// corsMiddleware is a stdlib-only replacement for github.com/go-chi/cors.
func corsMiddleware(app *app.Application) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed := false
			if app.Config.Env == "development" {
				allowed = true
			} else {
				for _, o := range app.Config.CORSAllowedOrigins {
					if origin == o {
						allowed = true
						break
					}
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Request-ID")
				w.Header().Set("Access-Control-Expose-Headers", "Link, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "300")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// recovererMiddleware catches panics, logs them, and returns a JSON
// envelope with the request ID.
func recovererMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID := middlewares.GetRequestID(r)
				log.Printf("[%s] 🔥 panic recovered: %v | %s %s",
					reqID, rec, r.Method, r.URL.Path)
				utils.ErrorJson(w, r, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
