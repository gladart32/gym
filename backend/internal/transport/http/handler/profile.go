// Package-level handler for the /api/v1/profile endpoints.
package handler

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/profile"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

const dateLayout = "2006-01-02"

// Profile exposes the /api/v1/profile endpoints.
type Profile struct {
	log *slog.Logger
	svc *profile.Service
}

func NewProfile(log *slog.Logger, svc *profile.Service) *Profile {
	return &Profile{log: log, svc: svc}
}

// Get handles GET /api/v1/profile: static fields plus the latest measurement.
func (h *Profile) Get(c *gin.Context) {
	userID := middleware.UserIDFrom(c)

	p, err := h.svc.Get(c.Request.Context(), userID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		p = nil // profile not filled yet — the client shows empty placeholders
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "get profile",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load profile")
		return
	}

	latest, err := h.svc.LatestMeasurement(c.Request.Context(), userID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		latest = nil
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "get latest measurement",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load measurements")
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"profile":             p,
		"current_measurement": latest,
	})
}

type updateProfileRequest struct {
	Gender         *string  `json:"gender"`
	BirthDate      *string  `json:"birth_date"`
	TargetWeightKg *float64 `json:"target_weight_kg"`
	FitnessLevel   *string  `json:"fitness_level"`
	TZOffset       *int     `json:"tz_offset"`
}

// Update handles PUT /api/v1/profile (upsert; every field is optional).
func (h *Profile) Update(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	loc, err := clientLocation(req.TZOffset)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	birthDate, err := parseDate(req.BirthDate, loc)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "birth_date must be YYYY-MM-DD")
		return
	}

	p, err := h.svc.Update(c.Request.Context(), middleware.UserIDFrom(c), &profile.Profile{
		Gender:         req.Gender,
		BirthDate:      birthDate,
		TargetWeightKg: req.TargetWeightKg,
		FitnessLevel:   req.FitnessLevel,
	})
	switch {
	case errors.Is(err, profile.ErrInvalidGender),
		errors.Is(err, profile.ErrInvalidFitnessLevel),
		errors.Is(err, profile.ErrInvalidBirthDate),
		errors.Is(err, profile.ErrInvalidTargetWeight):
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "update profile",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to update profile")
		return
	}
	response.Success(c, http.StatusOK, gin.H{"profile": p})
}

// ListMeasurements handles GET /api/v1/profile/measurements.
func (h *Profile) ListMeasurements(c *gin.Context) {
	items, err := h.svc.ListMeasurements(c.Request.Context(), middleware.UserIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "list measurements",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load measurements")
		return
	}
	response.Success(c, http.StatusOK, items)
}

type upsertMeasurementRequest struct {
	MeasuredOn string   `json:"measured_on" binding:"required"`
	WeightKg   *float64 `json:"weight_kg"`
	HeightCm   *float64 `json:"height_cm"`
	TZOffset   *int     `json:"tz_offset"`
}

// UpsertMeasurement handles POST /api/v1/profile/measurements: one entry
// per date, a repeated save for the same date overwrites it.
func (h *Profile) UpsertMeasurement(c *gin.Context) {
	var req upsertMeasurementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	loc, err := clientLocation(req.TZOffset)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	measuredOn, err := time.ParseInLocation(dateLayout, req.MeasuredOn, loc)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "measured_on must be YYYY-MM-DD")
		return
	}

	m, err := h.svc.UpsertMeasurement(c.Request.Context(), middleware.UserIDFrom(c), &profile.Measurement{
		MeasuredOn: measuredOn,
		WeightKg:   req.WeightKg,
		HeightCm:   req.HeightCm,
	})
	switch {
	case errors.Is(err, profile.ErrInvalidMeasuredOn),
		errors.Is(err, profile.ErrInvalidWeight),
		errors.Is(err, profile.ErrInvalidHeight),
		errors.Is(err, profile.ErrEmptyMeasurement):
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "upsert measurement",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to save measurement")
		return
	}
	response.Success(c, http.StatusOK, m)
}

// DeleteMeasurement handles DELETE /api/v1/profile/measurements/:id.
func (h *Profile) DeleteMeasurement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid measurement id")
		return
	}

	if err := h.svc.DeleteMeasurement(c.Request.Context(), middleware.UserIDFrom(c), id); err != nil {
		h.log.ErrorContext(c.Request.Context(), "delete measurement",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to delete measurement")
		return
	}
	response.Success(c, http.StatusOK, nil)
}

// Equipment handles GET /api/v1/profile/equipment: the full catalog
// plus the ids selected by the user.
func (h *Profile) Equipment(c *gin.Context) {
	sel, err := h.svc.Equipment(c.Request.Context(), middleware.UserIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "list equipment",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to load equipment")
		return
	}
	response.Success(c, http.StatusOK, sel)
}

type replaceEquipmentRequest struct {
	EquipmentIDs []uint16 `json:"equipment_ids"`
}

// ReplaceEquipment handles PUT /api/v1/profile/equipment.
func (h *Profile) ReplaceEquipment(c *gin.Context) {
	var req replaceEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	ids, err := h.svc.ReplaceEquipment(c.Request.Context(), middleware.UserIDFrom(c), req.EquipmentIDs)
	switch {
	case errors.Is(err, profile.ErrUnknownEquipment):
		response.Error(c, http.StatusBadRequest, profile.ErrUnknownEquipment.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "replace equipment",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to save equipment")
		return
	}
	if ids == nil {
		ids = []uint16{}
	}
	response.Success(c, http.StatusOK, gin.H{"selected_ids": ids})
}

// parseDate parses an optional YYYY-MM-DD string into a date pointer in loc.
func parseDate(s *string, loc *time.Location) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation(dateLayout, *s, loc)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// clientLocation builds a location from the browser's getTimezoneOffset()
// value (minutes behind UTC, e.g. Moscow → -180). nil or 0 means UTC.
func clientLocation(offset *int) (*time.Location, error) {
	if offset == nil || *offset == 0 {
		return time.UTC, nil
	}
	if *offset < -840 || *offset > 840 {
		return nil, errors.New("tz_offset must be within ±14 hours")
	}
	return time.FixedZone("client", -*offset*60), nil
}
