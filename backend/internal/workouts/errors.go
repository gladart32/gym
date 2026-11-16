package workouts

import "errors"

var (
	// ErrNotFound means the requested workout, exercise or set does not
	// exist (or is not owned by the requesting user).
	ErrNotFound = errors.New("workout entry not found")

	// ErrActiveExists means the user already has an unfinished workout
	// (enforced by uq_workouts_one_active).
	ErrActiveExists = errors.New("an active workout already exists")

	// ErrNotActive means the structural change is only allowed while the
	// workout is in progress.
	ErrNotActive = errors.New("workout is not in progress")

	// ErrInvalidValue is a field outside the allowed range.
	ErrInvalidValue = errors.New("invalid value")

	// ErrInvalidExercise is an unknown or inactive catalog exercise.
	ErrInvalidExercise = errors.New("exercise not found in catalog")

	// ErrTooManySets / ErrTooManyExercises guard against unreasonable
	// session growth.
	ErrTooManySets      = errors.New("too many sets in the exercise")
	ErrTooManyExercises = errors.New("too many exercises in the workout")
)
