package middlewares

import (
	"context"
	"net/http"
	"strings"

	"backend/internal/utils"
)

type contextKey string

const (
	UserIDKey   contextKey = "userID"
	UserRoleKey contextKey = "userRole"
)

func Authenticate(secret string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				utils.ErrorJson(w, r, http.StatusUnauthorized, "missing authorization header")
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				utils.ErrorJson(w, r, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			token := strings.TrimPrefix(authHeader, prefix)

			userID, role, err := utils.VerifyAccessToken(token, secret)
			if err != nil {
				utils.ErrorJson(w, r, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)

			next(w, r.WithContext(ctx))
		}
	}
}

func GetUserIDFromContext(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	return userID, ok
}

func GetUserRoleFromContext(r *http.Request) (string, bool) {
	role, ok := r.Context().Value(UserRoleKey).(string)
	return role, ok
}
