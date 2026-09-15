package utils

import (
	"net/http"
	"strconv"
)

// ============================================================
// PATH PARAMETERS
// ============================================================

// ParamID extracts and parses an integer ID from a URL path parameter.
// If no paramName is provided, defaults to "id".
func ParamID(w http.ResponseWriter, r *http.Request, paramName ...string) (int64, bool) {
	name := "id"
	if len(paramName) > 0 && paramName[0] != "" {
		name = paramName[0]
	}

	idStr := r.PathValue(name)
	if idStr == "" {
		ErrorJson(w, r, http.StatusBadRequest, "missing parameter: "+name)
		return 0, false
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ErrorJson(w, r, http.StatusBadRequest, "invalid parameter: "+name)
		return 0, false
	}

	return id, true
}

// ParamUUID extracts a UUID string from a URL path parameter.
// Validates the format but returns it as a string (keeps the
// stdlib-only constraint; pgx handles the DB-side conversion).
func ParamUUID(w http.ResponseWriter, r *http.Request, paramName ...string) (string, bool) {
	name := "id"
	if len(paramName) > 0 && paramName[0] != "" {
		name = paramName[0]
	}

	idStr := r.PathValue(name)
	if idStr == "" {
		ErrorJson(w, r, http.StatusBadRequest, "missing parameter: "+name)
		return "", false
	}

	if !IsValidUUID(idStr) {
		ErrorJson(w, r, http.StatusBadRequest, "invalid UUID parameter: "+name)
		return "", false
	}

	return idStr, true
}

// ============================================================
// UUID HELPERS
// ============================================================

// IsValidUUID returns true if s is a well-formed UUID string
// (8-4-4-4-12 hex chars).
func IsValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ============================================================
// QUERY PARAMETERS
// ============================================================

func QueryStr(r *http.Request, key string) (string, bool) {
	if v := r.URL.Query().Get(key); v != "" {
		return v, true
	}
	return "", false
}

func QueryInt(r *http.Request, key string) (int, bool) {
	if v := r.URL.Query().Get(key); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			return val, true
		}
	}
	return 0, false
}

func QueryInt64(r *http.Request, key string) (int64, bool) {
	if v := r.URL.Query().Get(key); v != "" {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			return val, true
		}
	}
	return 0, false
}

func QueryBool(r *http.Request, key string) (bool, bool) {
	if v := r.URL.Query().Get(key); v != "" {
		if val, err := strconv.ParseBool(v); err == nil {
			return val, true
		}
	}
	return false, false
}

func QueryFloat64(r *http.Request, key string) (float64, bool) {
	if v := r.URL.Query().Get(key); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			return val, true
		}
	}
	return 0, false
}
