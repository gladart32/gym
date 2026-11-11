// Handler for the /api/v1/programs endpoints.
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/exercises"
	"github.com/gladart32/gym/internal/programs"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

// Programs exposes the /api/v1/programs endpoints.
type Programs struct {
	log *slog.Logger
	svc *programs.Service
}

func NewPrograms(log *slog.Logger, svc *programs.Service) *Programs {
	return &Programs{log: log, svc: svc}
}

// List handles GET /api/v1/programs: ready and personal programs
// visible to the current user.
func (h *Programs) List(c *gin.Context) {
	f := &programs.ListFilter{
		Q:       c.Query("q"),
		UserID:  middleware.UserIDFrom(c),
		IsAdmin: middleware.RoleFrom(c) == auth.RoleAdmin,
	}

	scope, err := parseChoice(c.Query("scope"), "", "mine", "ready")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "scope must be mine or ready")
		return
	}
	f.Scope = scope

	level, err := parseChoice(c.Query("level"), "", "beginner", "intermediate", "advanced")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid level")
		return
	}
	f.Level = level

	f.Sort, err = parseChoice(c.Query("sort"), "newest", "newest", "name")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "sort must be newest or name")
		return
	}
	f.Order, err = parseChoice(c.Query("order"), "", "asc", "desc")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "order must be asc or desc")
		return
	}

	f.Page, err = parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid page")
		return
	}
	f.PerPage, err = parsePositiveInt(c.Query("per_page"), programs.DefaultPerPage)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid per_page")
		return
	}

	res, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "list programs",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load programs")
		return
	}
	response.Success(c, http.StatusOK, res)
}

// Detail handles GET /api/v1/programs/:id: the full program tree.
func (h *Programs) Detail(c *gin.Context) {
	id, ok := parseProgramID(c)
	if !ok {
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	d, err := h.svc.Detail(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), id, lang)
	if err != nil {
		h.domainError(c, err, "failed to load program")
		return
	}
	response.Success(c, http.StatusOK, d)
}

// Create handles POST /api/v1/programs: a personal program, or a ready
// program when published by an admin.
func (h *Programs) Create(c *gin.Context) {
	var in programs.Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid body")
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	id, err := h.svc.Create(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), lang, &in)
	if err != nil {
		h.domainError(c, err, "failed to create program")
		return
	}

	d, err := h.svc.Detail(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), id, lang)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "load created program",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load program")
		return
	}
	response.Success(c, http.StatusOK, d)
}

// Update handles PUT /api/v1/programs/:id: full header and structure
// replacement (owner of a personal program or admin of a ready one).
func (h *Programs) Update(c *gin.Context) {
	id, ok := parseProgramID(c)
	if !ok {
		return
	}
	var in programs.Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid body")
		return
	}

	if err := h.svc.Update(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), id, &in); err != nil {
		h.domainError(c, err, "failed to update program")
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	d, err := h.svc.Detail(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), id, lang)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "load updated program",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load program")
		return
	}
	response.Success(c, http.StatusOK, d)
}

// Delete handles DELETE /api/v1/programs/:id.
func (h *Programs) Delete(c *gin.Context) {
	id, ok := parseProgramID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), id); err != nil {
		h.domainError(c, err, "failed to delete program")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// Copy handles POST /api/v1/programs/:id/copy: duplicates a visible
// program as a personal one of the current user.
func (h *Programs) Copy(c *gin.Context) {
	id, ok := parseProgramID(c)
	if !ok {
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	newID, err := h.svc.Copy(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), id)
	if err != nil {
		h.domainError(c, err, "failed to copy program")
		return
	}

	d, err := h.svc.Detail(c.Request.Context(), middleware.UserIDFrom(c), isAdmin(c), newID, lang)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "load copied program",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load program")
		return
	}
	response.Success(c, http.StatusOK, d)
}

// domainError maps domain sentinel errors to HTTP responses.
func (h *Programs) domainError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, programs.ErrNotFound):
		response.Error(c, http.StatusNotFound, programs.ErrNotFound.Error())
		return
	case errors.Is(err, programs.ErrForbidden):
		response.Error(c, http.StatusForbidden, programs.ErrForbidden.Error())
		return
	case errors.Is(err, programs.ErrInvalidName),
		errors.Is(err, programs.ErrInvalidLevel),
		errors.Is(err, programs.ErrInvalidDayName),
		errors.Is(err, programs.ErrTooManyItems),
		errors.Is(err, programs.ErrInvalidSetNumber),
		errors.Is(err, programs.ErrDuplicateSetNumber),
		errors.Is(err, programs.ErrInvalidWeight),
		errors.Is(err, programs.ErrUnknownExercise):
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	h.log.ErrorContext(c.Request.Context(), fallback,
		"error", err, "request_id", middleware.RequestIDFrom(c))
	response.Error(c, http.StatusInternalServerError, fallback)
}

// parseProgramID extracts and validates the :id route parameter.
func parseProgramID(c *gin.Context) (uint32, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid program id")
		return 0, false
	}
	return uint32(id), true
}

func isAdmin(c *gin.Context) bool {
	return middleware.RoleFrom(c) == auth.RoleAdmin
}
