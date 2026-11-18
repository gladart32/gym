// Handler for the /api/v1/stats endpoints.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/stats"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

// Stats exposes the /api/v1/stats endpoints.
type Stats struct {
	log *slog.Logger
	svc *stats.Service
}

func NewStats(log *slog.Logger, svc *stats.Service) *Stats {
	return &Stats{log: log, svc: svc}
}

// Dashboard handles GET /api/v1/stats/dashboard: all statistics
// for the main page (counters, streak, weekly activity, last workout).
func (h *Stats) Dashboard(c *gin.Context) {
	d, err := h.svc.Dashboard(c.Request.Context(), middleware.UserIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "load dashboard stats",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load stats")
		return
	}
	response.Success(c, http.StatusOK, d)
}
