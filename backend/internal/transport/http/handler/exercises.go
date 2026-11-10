// Handler for the /api/v1/exercises endpoints.
package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/exercises"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

// Exercises exposes the /api/v1/exercises endpoints.
type Exercises struct {
	log *slog.Logger
	svc *exercises.Service
}

func NewExercises(log *slog.Logger, svc *exercises.Service) *Exercises {
	return &Exercises{log: log, svc: svc}
}

const maxFilterIDs = 100

// List handles GET /api/v1/exercises: filtered, sorted, paginated catalog.
func (h *Exercises) List(c *gin.Context) {
	f := &exercises.ListFilter{
		Q:           c.Query("q"),
		MyEquipment: c.Query("my_equipment") == "1" || c.Query("my_equipment") == "true",
		UserID:      middleware.UserIDFrom(c),
		Lang:        exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language")),
	}

	for name, dst := range map[string]*[]uint16{
		"body_part_ids": &f.BodyPartIDs,
		"equipment_ids": &f.EquipmentIDs,
		"muscle_ids":    &f.MuscleIDs,
	} {
		ids, err := parseIDSlice(c.Query(name), maxFilterIDs)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Sprintf("invalid %s", name))
			return
		}
		*dst = ids
	}

	tts, err := parseTrackingTypes(c.Query("tracking_types"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	f.TrackingTypes = tts

	f.Sort, err = parseChoice(c.Query("sort"), "name", "name", "newest")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "sort must be name or newest")
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
	f.PerPage, err = parsePositiveInt(c.Query("per_page"), exercises.DefaultPerPage)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid per_page")
		return
	}

	res, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "list exercises",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load exercises")
		return
	}
	response.Success(c, http.StatusOK, res)
}

// Detail handles GET /api/v1/exercises/:id: the full exercise card.
func (h *Exercises) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid exercise id")
		return
	}

	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	d, err := h.svc.Detail(c.Request.Context(), uint32(id), lang)
	switch {
	case errors.Is(err, exercises.ErrNotFound):
		response.Error(c, http.StatusNotFound, exercises.ErrNotFound.Error())
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "exercise detail",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load exercise")
	default:
		response.Success(c, http.StatusOK, d)
	}
}

// Filters handles GET /api/v1/exercises/filters: localized dictionaries.
func (h *Exercises) Filters(c *gin.Context) {
	lang := exercises.LangFromAcceptLanguage(c.GetHeader("Accept-Language"))
	f, err := h.svc.Filters(c.Request.Context(), lang)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "exercise filters",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load filters")
		return
	}
	response.Success(c, http.StatusOK, f)
}

// parseIDSlice splits a comma-separated list of unsigned ids.
func parseIDSlice(s string, limit int) ([]uint16, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) > limit {
		return nil, errors.New("too many ids")
	}
	ids := make([]uint16, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseUint(strings.TrimSpace(p), 10, 16)
		if err != nil {
			return nil, err
		}
		ids = append(ids, uint16(v))
	}
	return ids, nil
}

func parseTrackingTypes(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !exercises.IsTrackingType(p) {
			return nil, errors.New("invalid tracking_types")
		}
		out = append(out, p)
	}
	return out, nil
}

// parseChoice validates an optional enum query value with a default.
func parseChoice(value, empty string, allowed ...string) (string, error) {
	if value == "" {
		return empty, nil
	}
	for _, a := range allowed {
		if value == a {
			return value, nil
		}
	}
	return "", errors.New("invalid value")
}

func parsePositiveInt(s string, empty int) (int, error) {
	if s == "" {
		return empty, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return 0, errors.New("invalid value")
	}
	return v, nil
}
