package programs

import "errors"

var (
	// ErrNotFound means the program does not exist or is not visible
	// to the requesting user (declared in detail.go).

	// ErrForbidden means the program exists but the user may not modify it
	// (declared in detail.go).

	// ErrInvalidName is an empty or overlong program name.
	ErrInvalidName = errors.New("name must be 1..255 characters")

	// ErrInvalidLevel is a level outside the catalog values.
	ErrInvalidLevel = errors.New("level must be beginner, intermediate or advanced")

	// ErrInvalidDayName is an empty or overlong day name.
	ErrInvalidDayName = errors.New("day name must be 1..255 characters")

	// ErrTooManyItems exceeds the days/exercises/sets structural limits.
	ErrTooManyItems = errors.New("too many days, exercises or sets")

	// ErrInvalidSetNumber is a set number outside 1..255.
	ErrInvalidSetNumber = errors.New("set_number must be 1..255")

	// ErrDuplicateSetNumber repeats a set number within one exercise.
	ErrDuplicateSetNumber = errors.New("set_number must be unique within an exercise")

	// ErrInvalidWeight is a weight outside the DECIMAL(6,2) sanity bounds.
	ErrInvalidWeight = errors.New("weight_kg must be between -300 and 500")

	// ErrUnknownExercise references an exercise missing from the active catalog.
	ErrUnknownExercise = errors.New("days contain unknown or inactive exercise ids")
)
