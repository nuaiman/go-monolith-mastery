package middlewares

import (
	"log"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func (rw *responseWriter) Status() int {
	return rw.statusCode
}

func (rw *responseWriter) BytesWritten() int64 {
	return rw.written
}

// RequestLogger logs the request ID, method, path, status, duration,
// size, and client IP for every request.
//
// Assumes RequestID middleware has already run so the ID is available.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			written:        0,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		reqID := GetRequestID(r)

		log.Printf("[%s] %s %s | Status: %d | Duration: %v | Size: %d bytes | IP: %s",
			reqID,
			r.Method,
			r.URL.Path,
			rw.Status(),
			duration,
			rw.BytesWritten(),
			r.RemoteAddr,
		)
	})
}
