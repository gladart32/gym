package profile

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Profile holds the user profile fields. Gender/BirthDate/TargetWeightKg/
// FitnessLevel are optional: the row is created on the first save (upsert
// by primary key user_id). Timezone lives in users.timezone and is always
// present (column has a default).
type Profile struct {
	Gender         *string    `json:"gender" db:"gender"`
	BirthDate      *time.Time `json:"birth_date" db:"birth_date"`
	TargetWeightKg *float64   `json:"target_weight_kg" db:"target_weight_kg"`
	FitnessLevel   *string    `json:"fitness_level" db:"fitness_level"`
	Timezone       *string    `json:"timezone" db:"timezone"`
}

var (
	ErrInvalidGender       = errors.New("gender must be male or female")
	ErrInvalidFitnessLevel = errors.New("fitness_level must be beginner, intermediate or advanced")
	ErrInvalidBirthDate    = errors.New("birth_date must be in the past")
	ErrInvalidTargetWeight = errors.New("target_weight_kg must be between 20 and 500")
)

const (
	getProfileQuery = `
		SELECT gender, birth_date, target_weight_kg, fitness_level
		FROM user_profiles WHERE user_id = ?`

	getTimezoneQuery = `
		SELECT timezone FROM users WHERE id = ?`

	updateTimezoneQuery = `
		UPDATE users SET timezone = ? WHERE id = ?`

	upsertProfileQuery = `
		INSERT INTO user_profiles (user_id, gender, birth_date, target_weight_kg, fitness_level)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			gender = VALUES(gender),
			birth_date = VALUES(birth_date),
			target_weight_kg = VALUES(target_weight_kg),
			fitness_level = VALUES(fitness_level)`
)

// Get returns the profile fields; the timezone always comes from users.
// Unfilled profile fields stay nil.
func (s *Service) Get(ctx context.Context, userID uint32) (*Profile, error) {
	tz, err := s.timezoneOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	p := &Profile{Timezone: &tz}

	var row Profile
	err = s.db.GetContext(ctx, &row, getProfileQuery, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil // profile not filled yet
	}
	if err != nil {
		return nil, err
	}
	p.Gender, p.BirthDate, p.TargetWeightKg, p.FitnessLevel =
		row.Gender, row.BirthDate, row.TargetWeightKg, row.FitnessLevel
	return p, nil
}

// Update validates, upserts the profile row and optionally replaces the
// user's timezone (nil = keep the current one).
func (s *Service) Update(ctx context.Context, userID uint32, p *Profile) (*Profile, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	if p.Timezone != nil {
		loc, err := NormalizeTimezone(*p.Timezone)
		if err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx, updateTimezoneQuery, TimezoneName(loc), userID); err != nil {
			return nil, err
		}
	}

	birthDate := toDate(p.BirthDate)
	if _, err := s.db.ExecContext(ctx, upsertProfileQuery,
		userID, p.Gender, birthDate, p.TargetWeightKg, p.FitnessLevel); err != nil {
		return nil, err
	}

	return s.Get(ctx, userID)
}

// timezoneOf returns the stored IANA timezone name (never empty: the
// column has a default).
func (s *Service) timezoneOf(ctx context.Context, userID uint32) (string, error) {
	var tz string
	if err := s.db.GetContext(ctx, &tz, getTimezoneQuery, userID); err != nil {
		return "", err
	}
	return tz, nil
}

func (p *Profile) validate() error {
	if p.Gender != nil && *p.Gender != "male" && *p.Gender != "female" {
		return ErrInvalidGender
	}
	if p.FitnessLevel != nil && *p.FitnessLevel != "beginner" &&
		*p.FitnessLevel != "intermediate" && *p.FitnessLevel != "advanced" {
		return ErrInvalidFitnessLevel
	}
	if p.BirthDate != nil && p.BirthDate.After(endOfToday(p.BirthDate.Location())) {
		return ErrInvalidBirthDate
	}
	if p.TargetWeightKg != nil && (*p.TargetWeightKg < 20 || *p.TargetWeightKg > 500) {
		return ErrInvalidTargetWeight
	}
	return nil
}

// toDate normalizes a date to midnight UTC keeping its calendar Y/M/D:
// converting through UTC first would shift the date for clients east of UTC.
func toDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &d
}
