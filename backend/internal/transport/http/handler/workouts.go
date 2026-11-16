// Handler for the /api/v1/workouts endpoints.
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/exercises"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
	"github.com/gladart32/gym/internal/workouts"
)

// Workouts exposes the /api/v1/workouts endpoints.
type Workouts struct {
	log *slog.Logger
	svc *workouts.Service
}

func NewWorkouts(log *slog.Logger, svc *workouts.Service) *Workouts {
	return &Workouts{log: log, svc: svc}
}

// Start handles POST /api/v1/workouts/start: creates a workout from a
// program day (plan snapshot) and returns the session.
func (h *Workouts) Start(c *gin.Context) {
	var body struct {
		UserProgramID uint32 `json:"user_program_id"`
		ProgramDayID  uint32 `json:"program_day_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UserProgramID == 0 || body.ProgramDayID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid body: user_program_id and program_day_id are required")
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	session, err := h.svc.Start(c.Request.Context(), middleware.UserIDFrom(c), body.UserProgramID, body.ProgramDayID, lang)
	if err != nil {
		h.domainError(c, err, "failed to start workout")
		return
	}
	response.Success(c, http.StatusCreated, session)
}

// Active handles GET /api/v1/workouts/active: the user's unfinished
// workout or null.
func (h *Workouts) Active(c *gin.Context) {
	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	session, err := h.svc.Active(c.Request.Context(), middleware.UserIDFrom(c), lang)
	if err != nil {
		h.domainError(c, err, "failed to load active workout")
		return
	}
	response.Success(c, http.StatusOK, session)
}

// UpdateSet handles PATCH /api/v1/workouts/sets/:id: replaces the fact.
func (h *Workouts) UpdateSet(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid set id")
		return
	}

	var in workouts.SetInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid body")
		return
	}

	if err := h.svc.UpdateSet(c.Request.Context(), middleware.UserIDFrom(c), uint32(id), &in); err != nil {
		h.domainError(c, err, "failed to update set")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// AddSet handles POST /api/v1/workouts/exercises/:id/sets.
func (h *Workouts) AddSet(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid exercise id")
		return
	}

	set, err := h.svc.AddSet(c.Request.Context(), middleware.UserIDFrom(c), uint32(id))
	if err != nil {
		h.domainError(c, err, "failed to add set")
		return
	}
	response.Success(c, http.StatusCreated, set)
}

// UpdateExercise handles PATCH /api/v1/workouts/exercises/:id
// (skip/unskip, replace the catalog exercise).
func (h *Workouts) UpdateExercise(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid exercise id")
		return
	}

	var in workouts.ExerciseInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid body")
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	exercise, err := h.svc.UpdateExercise(c.Request.Context(), middleware.UserIDFrom(c), uint32(id), lang, &in)
	if err != nil {
		h.domainError(c, err, "failed to update exercise")
		return
	}
	response.Success(c, http.StatusOK, exercise)
}

// AddExercise handles POST /api/v1/workouts/exercises.
func (h *Workouts) AddExercise(c *gin.Context) {
	var in workouts.AddExerciseInput
	if err := c.ShouldBindJSON(&in); err != nil || in.ExerciseID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid body: exercise_id is required")
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	exercise, err := h.svc.AddExercise(c.Request.Context(), middleware.UserIDFrom(c), lang, &in)
	if err != nil {
		h.domainError(c, err, "failed to add exercise")
		return
	}
	response.Success(c, http.StatusCreated, exercise)
}

// Complete handles POST /api/v1/workouts/complete.
func (h *Workouts) Complete(c *gin.Context) {
	if err := h.svc.Complete(c.Request.Context(), middleware.UserIDFrom(c)); err != nil {
		h.domainError(c, err, "failed to complete workout")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// Cancel handles POST /api/v1/workouts/cancel.
func (h *Workouts) Cancel(c *gin.Context) {
	if err := h.svc.Cancel(c.Request.Context(), middleware.UserIDFrom(c)); err != nil {
		h.domainError(c, err, "failed to cancel workout")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// domainError maps workouts domain errors to HTTP responses.
func (h *Workouts) domainError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, workouts.ErrNotFound):
		response.Error(c, http.StatusNotFound, workouts.ErrNotFound.Error())
		return
	case errors.Is(err, workouts.ErrActiveExists):
		response.Error(c, http.StatusConflict, workouts.ErrActiveExists.Error())
		return
	case errors.Is(err, workouts.ErrNotActive):
		response.Error(c, http.StatusConflict, workouts.ErrNotActive.Error())
		return
	case errors.Is(err, workouts.ErrInvalidValue),
		errors.Is(err, workouts.ErrInvalidExercise):
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, workouts.ErrTooManySets),
		errors.Is(err, workouts.ErrTooManyExercises):
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}

	h.log.ErrorContext(c.Request.Context(), fallback,
		"error", err, "request_id", middleware.RequestIDFrom(c))
	response.Error(c, http.StatusInternalServerError, fallback)
}
