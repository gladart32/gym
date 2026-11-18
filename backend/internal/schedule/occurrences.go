package schedule

import (
	"context"
	"time"
)

// Occurrence is one scheduled workout-day occurrence on a concrete date,
// used by stats to compare the plan with actually completed workouts.
type Occurrence struct {
	Date          string `json:"date"` // YYYY-MM-DD
	UserProgramID uint32 `json:"user_program_id"`
	ProgramDayID  uint32 `json:"program_day_id"`
	DayName       string `json:"day_name"`
}

// Occurrences returns every scheduled occurrence of the user's active
// programs on dates in [from..to] (inclusive, date-only).
func (s *Service) Occurrences(ctx context.Context, userID uint32, from, to time.Time) ([]Occurrence, error) {
	ups, err := s.loadUserPrograms(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := []Occurrence{}
	for d := dateOnly(from); !d.After(dateOnly(to)); d = d.AddDate(0, 0, 1) {
		for _, e := range entriesFor(ups, d) {
			out = append(out, Occurrence{
				Date:          d.Format("2006-01-02"),
				UserProgramID: e.UserProgramID,
				ProgramDayID:  e.ProgramDayID,
				DayName:       e.DayName,
			})
		}
	}
	return out, nil
}
