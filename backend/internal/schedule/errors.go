package schedule

import (
	"errors"

	"github.com/gladart32/gym/internal/programs"
)

var (
	// ErrNotFound means the user program or rule does not exist
	// (or is not visible to the requesting user).
	ErrNotFound = errors.New("schedule entry not found")

	// ErrForbidden means the entry exists but belongs to another user.
	ErrForbidden = errors.New("forbidden: this schedule belongs to another user")

	// ErrInvalidStartDate is a malformed start_date (expected YYYY-MM-DD).
	ErrInvalidStartDate = errors.New("invalid start_date: expected YYYY-MM-DD")

	// ErrInvalidDate is a malformed week anchor date (expected YYYY-MM-DD).
	ErrInvalidDate = errors.New("invalid date: expected YYYY-MM-DD")

	// ErrInvalidRule is a malformed recurrence rule; programs.ErrInvalidSchedule
	// is reused so both handlers map it to 400 the same way.
	_ error = programs.ErrInvalidSchedule
)
