package profile

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Profile holds the static user profile fields. Every field is optional:
// the row is created on the first save (upsert by primary key user_id).
type Profile struct {
	Gender         *string    `json:"gender" db:"gender"`
	BirthDate      *time.Time `json:"birth_date" db:"birth_date"`
	TargetWeightKg *float64   `json:"target_weight_kg" db:"target_weight_kg"`
	FitnessLevel   *string    `json:"fitness_level" db:"fitness_level"`
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

	upsertProfileQuery = `
		INSERT INTO user_profiles (user_id, gender, birth_date, target_weight_kg, fitness_level)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			gender = VALUES(gender),
			birth_date = VALUES(birth_date),
			target_weight_kg = VALUES(target_weight_kg),
			fitness_level = VALUES(fitness_level)`
)

// Get returns the user profile or sql.ErrNoRows when it was never saved.
func (s *Service) Get(ctx context.Context, userID uint32) (*Profile, error) {
	var p Profile
	err := s.db.GetContext(ctx, &p, getProfileQuery, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Update validates and upserts the profile row for the user.
func (s *Service) Update(ctx context.Context, userID uint32, p *Profile) (*Profile, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	birthDate := toDate(p.BirthDate)
	if _, err := s.db.ExecContext(ctx, upsertProfileQuery,
		userID, p.Gender, birthDate, p.TargetWeightKg, p.FitnessLevel); err != nil {
		return nil, err
	}

	return s.Get(ctx, userID)
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
