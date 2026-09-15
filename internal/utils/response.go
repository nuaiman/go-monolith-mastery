// internal/utils/response.go
package utils

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"backend/internal/constants"
)

// Response is the uniform JSON envelope for every API response.
// request_id is included whenever the RequestID middleware has run.
type Response struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ReadJson decodes a single JSON object from the request body into dst.
// Rejects unknown fields and enforces a 1 MB body cap.
func ReadJson(w http.ResponseWriter, r *http.Request, dst any) error {
	const maxBytes = 1_048_576 // 1 MB

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&dst); err != nil {
		return err
	}

	// Ensure only one JSON object
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("body must only contain a single JSON object")
	}

	return nil
}

// SuccessJson writes a JSON success envelope.
func SuccessJson(w http.ResponseWriter, r *http.Request, status int, message string, data any) error {
	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		RequestID: requestIDFromContext(r),
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(jsonData)
	return err
}

// ErrorJson writes a JSON error envelope.
func ErrorJson(w http.ResponseWriter, r *http.Request, status int, message string) {
	response := Response{
		Success:   false,
		Message:   message,
		RequestID: requestIDFromContext(r),
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"success":false,"message":"internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(jsonData)
}

func requestIDFromContext(r *http.Request) string {
	if r == nil {
		return ""
	}
	id, _ := r.Context().Value(constants.CtxRequestID).(string)
	return id
}
