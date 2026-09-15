package middlewares

import (
	"net/http"

	"backend/internal/utils"
)

// RequireRole rejects the request unless the authenticated user has one
// of the allowed roles. Must be used after Authenticate.
//
// Usage (callers import backend/internal/constants for role names):
//
//	protected := middlewares.Authenticate(cfg.JWTKey)
//	superOnly := protected(middlewares.RequireRole(constants.RoleSuperAdmin))
//	adminOrSuper := protected(middlewares.RequireRole(constants.RoleAdmin, constants.RoleSuperAdmin))
func RequireRole(allowedRoles ...string) func(http.HandlerFunc) http.HandlerFunc {
	allowed := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = true
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			role, ok := GetUserRoleFromContext(r)
			if !ok {
				utils.ErrorJson(w, r, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !allowed[role] {
				utils.ErrorJson(w, r, http.StatusForbidden, "insufficient permissions")
				return
			}
			next(w, r)
		}
	}
}
