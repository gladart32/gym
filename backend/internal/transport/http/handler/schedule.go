// Handler for the /api/v1/schedule endpoints and program selection.
package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/programs"
	"github.com/gladart32/gym/internal/schedule"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

// Schedule exposes the /api/v1/schedule endpoints.
type Schedule struct {
	log *slog.Logger
	svc *schedule.Service
}

func NewSchedule(log *slog.Logger, svc *schedule.Service) *Schedule {
	return &Schedule{log: log, svc: svc}
}

// Select handles POST /api/v1/programs/:id/select: activates the program
// for the current user and copies its schedule rules.
func (h *Schedule) Select(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid program id")
		return
	}

	var body struct {
		StartDate *string `json:"start_date"`
	}
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		response.Error(c, http.StatusBadRequest, "invalid body")
		return
	}

	up, err := h.svc.Select(c.Request.Context(), middleware.UserIDFrom(c), uint32(id), isAdmin(c), body.StartDate)
	if err != nil {
		h.domainError(c, err, "failed to select program")
		return
	}
	response.Success(c, http.StatusOK, up)
}

// Get handles GET /api/v1/schedule: the user's active programs with rules.
func (h *Schedule) Get(c *gin.Context) {
	ups, err := h.svc.Get(c.Request.Context(), middleware.UserIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "load schedule",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load schedule")
		return
	}
	response.Success(c, http.StatusOK, ups)
}

// UpdateRule handles PUT /api/v1/schedule/rules/:id.
func (h *Schedule) UpdateRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid rule id")
		return
	}

	var in schedule.RuleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid body")
		return
	}

	rule, err := h.svc.UpdateRule(c.Request.Context(), middleware.UserIDFrom(c), uint32(id), &in)
	if err != nil {
		h.domainError(c, err, "failed to update schedule rule")
		return
	}
	response.Success(c, http.StatusOK, rule)
}

// DeleteRule handles DELETE /api/v1/schedule/rules/:id.
func (h *Schedule) DeleteRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid rule id")
		return
	}

	if err := h.svc.DeleteRule(c.Request.Context(), middleware.UserIDFrom(c), uint32(id)); err != nil {
		h.domainError(c, err, "failed to delete schedule rule")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// Deactivate handles DELETE /api/v1/schedule/programs/:id.
func (h *Schedule) Deactivate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user program id")
		return
	}

	if err := h.svc.Deactivate(c.Request.Context(), middleware.UserIDFrom(c), uint32(id)); err != nil {
		h.domainError(c, err, "failed to deactivate program")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// Week handles GET /api/v1/schedule/week?date=YYYY-MM-DD: the merged plan
// of all active user programs for the week containing the date.
func (h *Schedule) Week(c *gin.Context) {
	plan, err := h.svc.Week(c.Request.Context(), middleware.UserIDFrom(c), c.Query("date"))
	if err != nil {
		h.domainError(c, err, "failed to load week plan")
		return
	}
	response.Success(c, http.StatusOK, plan)
}

// Next handles GET /api/v1/schedule/next: the closest scheduled workout
// on or after today.
func (h *Schedule) Next(c *gin.Context) {
	next, err := h.svc.Next(c.Request.Context(), middleware.UserIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "load next workout",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load next workout")
		return
	}
	response.Success(c, http.StatusOK, next)
}

// domainError maps schedule domain errors to HTTP responses.
func (h *Schedule) domainError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, schedule.ErrNotFound):
		response.Error(c, http.StatusNotFound, schedule.ErrNotFound.Error())
		return
	case errors.Is(err, schedule.ErrForbidden):
		response.Error(c, http.StatusForbidden, schedule.ErrForbidden.Error())
		return
	case errors.Is(err, schedule.ErrInvalidStartDate),
		errors.Is(err, schedule.ErrInvalidDate),
		errors.Is(err, programs.ErrInvalidSchedule):
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	h.log.ErrorContext(c.Request.Context(), fallback,
		"error", err, "request_id", middleware.RequestIDFrom(c))
	response.Error(c, http.StatusInternalServerError, fallback)
}
