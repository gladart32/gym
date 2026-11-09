package profile

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Measurement is one body measurement entry. weight_kg and height_cm are
// independent: either may be NULL, but at least one must be set (CHECK).
type Measurement struct {
	ID         uint64    `json:"id" db:"id"`
	MeasuredOn time.Time `json:"measured_on" db:"measured_on"`
	WeightKg   *float64  `json:"weight_kg" db:"weight_kg"`
	HeightCm   *float64  `json:"height_cm" db:"height_cm"`
}

var (
	ErrInvalidMeasuredOn = errors.New("measured_on must be a valid date not in the future")
	ErrInvalidWeight     = errors.New("weight_kg must be between 20 and 500")
	ErrInvalidHeight     = errors.New("height_cm must be between 50 and 300")
	ErrEmptyMeasurement  = errors.New("measurement must include weight_kg or height_cm")
)

const (
	listMeasurementsQuery = `
		SELECT id, measured_on, weight_kg, height_cm
		FROM body_measurements
		WHERE user_id = ?
		ORDER BY measured_on DESC`

	upsertMeasurementQuery = `
		INSERT INTO body_measurements (user_id, measured_on, weight_kg, height_cm)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			weight_kg = VALUES(weight_kg),
			height_cm = VALUES(height_cm)`

	selectUpsertedMeasurementQuery = `
		SELECT id, measured_on, weight_kg, height_cm
		FROM body_measurements
		WHERE user_id = ? AND measured_on = ?`

	deleteMeasurementQuery = `
		DELETE FROM body_measurements WHERE id = ? AND user_id = ?`

	latestMeasurementQuery = `
		SELECT id, measured_on, weight_kg, height_cm
		FROM body_measurements
		WHERE user_id = ?
		ORDER BY measured_on DESC
		LIMIT 1`
)

// List returns the user's measurement history, newest first.
func (s *Service) ListMeasurements(ctx context.Context, userID uint32) ([]Measurement, error) {
	var items []Measurement
	if err := s.db.SelectContext(ctx, &items, listMeasurementsQuery, userID); err != nil {
		return nil, err
	}
	if items == nil {
		items = []Measurement{}
	}
	return items, nil
}

// Latest returns the most recent measurement or sql.ErrNoRows.
func (s *Service) LatestMeasurement(ctx context.Context, userID uint32) (*Measurement, error) {
	var m Measurement
	err := s.db.GetContext(ctx, &m, latestMeasurementQuery, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpsertMeasurement saves the value for the given date: one entry per day,
// a repeated save for the same date overwrites the previous one.
func (s *Service) UpsertMeasurement(ctx context.Context, userID uint32, m *Measurement) (*Measurement, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}

	measuredOn := toDate(&m.MeasuredOn)
	if _, err := s.db.ExecContext(ctx, upsertMeasurementQuery,
		userID, measuredOn, m.WeightKg, m.HeightCm); err != nil {
		return nil, err
	}

	var saved Measurement
	if err := s.db.GetContext(ctx, &saved, selectUpsertedMeasurementQuery, userID, measuredOn); err != nil {
		return nil, err
	}
	return &saved, nil
}

// DeleteMeasurement removes the entry if it belongs to the user.
func (s *Service) DeleteMeasurement(ctx context.Context, userID uint32, id uint64) error {
	_, err := s.db.ExecContext(ctx, deleteMeasurementQuery, id, userID)
	return err
}

func (m *Measurement) validate() error {
	if m.MeasuredOn.IsZero() || m.MeasuredOn.After(endOfToday(m.MeasuredOn.Location())) {
		return ErrInvalidMeasuredOn
	}
	if m.WeightKg != nil && (*m.WeightKg < 20 || *m.WeightKg > 500) {
		return ErrInvalidWeight
	}
	if m.HeightCm != nil && (*m.HeightCm < 50 || *m.HeightCm > 300) {
		return ErrInvalidHeight
	}
	if m.WeightKg == nil && m.HeightCm == nil {
		return ErrEmptyMeasurement
	}
	return nil
}

// endOfToday returns the end of the current calendar day in loc. Dates are
// parsed in the client's timezone, so "today" is judged by the client's
// calendar, not the server's.
func endOfToday(loc *time.Location) time.Time {
	y, mth, d := time.Now().In(loc).Date()
	return time.Date(y, mth, d, 23, 59, 59, int(time.Second)-1, loc)
}
