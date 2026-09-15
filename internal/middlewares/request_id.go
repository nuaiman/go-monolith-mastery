package middlewares

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"backend/internal/constants"
)

// RequestID generates (or reads) a short correlation ID per request.
// It sets:
//
//   - X-Request-ID response header
//   - constants.CtxRequestID in the request context
//
// If the client supplies X-Request-ID, it is trusted and reused. This
// lets a frontend correlate its own logs with the server's.
//
// Must run early in the middleware chain, before any logging.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}

		w.Header().Set("X-Request-ID", id)

		ctx := context.WithValue(r.Context(), constants.CtxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from a request's context.
// Returns "" if RequestID middleware did not run.
func GetRequestID(r *http.Request) string {
	id, _ := r.Context().Value(constants.CtxRequestID).(string)
	return id
}

// generateRequestID returns 16 hex chars (8 bytes of entropy).
// Collision-free in practice.
func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
