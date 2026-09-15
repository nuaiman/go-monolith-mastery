package handlers

import (
	"net/http"
	"time"

	"backend/internal/utils"
)

// HealthzHandler godoc
// @Summary      Service health
// @Tags         healthz
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /healthz [get]
func (h *Handler) HealthzHandler(w http.ResponseWriter, r *http.Request) {
	dbErr := h.app.DB.Ping(r.Context())

	status := "ok"
	if dbErr != nil {
		status = "degraded"
	}

	utils.SuccessJson(w, r, http.StatusOK, "service status", map[string]any{
		"status": status,
		"database": map[string]any{
			"connected": dbErr == nil,
		},
		"timestamp": time.Now(),
	})
}
