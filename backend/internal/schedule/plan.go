package schedule

import (
	"context"
	"sort"
	"time"
)

// weekdayCodes maps time.Weekday to the rule/storage codes.
var weekdayCodes = map[time.Weekday]string{
	time.Monday:    "mon",
	time.Tuesday:   "tue",
	time.Wednesday: "wed",
	time.Thursday:  "thu",
	time.Friday:    "fri",
	time.Saturday:  "sat",
	time.Sunday:    "sun",
}

// splitWeekdays splits a MySQL SET value ("mon,wed,fri") into a slice.
func splitWeekdays(s string) []string {
	out := make([]string, 0, 7)
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if part := s[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

// weekdaySet converts a weekday list into a lookup set.
func weekdaySet(days []string) map[string]struct{} {
	set := make(map[string]struct{}, len(days))
	for _, d := range days {
		set[d] = struct{}{}
	}
	return set
}

// dateOnly truncates a time to the UTC midnight of its date.
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// today returns today's date (UTC midnight).
func today() time.Time {
	return dateOnly(time.Now().UTC())
}

// mondayOf returns the Monday of the week containing t (Monday-based week).
func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Mon -> 0 .. Sun -> 6
	return dateOnly(t).AddDate(0, 0, -offset)
}

// daysBetween returns the whole-day difference d - start (both date-only).
func daysBetween(start, d time.Time) int {
	return int(d.Sub(start).Hours() / 24)
}

// ruleMatches reports whether the rule fires on date d.
// start is the user program's start_date anchor (date-only, UTC).
func ruleMatches(rule Rule, start time.Time, d time.Time) bool {
	switch rule.Frequency {
	case "once":
		return rule.OnceDate != nil && *rule.OnceDate == d.Format("2006-01-02")
	case "daily":
		diff := daysBetween(start, d)
		if diff < 0 {
			return false
		}
		return uint(diff)%uint(max8(rule.IntervalN, 1)) == 0
	case "weekly":
		if _, ok := weekdaySet(rule.Weekdays)[weekdayCodes[d.Weekday()]]; !ok {
			return false
		}
		diff := daysBetween(start, d)
		if diff < 0 {
			return false
		}
		return uint(diff/7)%uint(max8(rule.IntervalN, 1)) == 0
	default:
		return false
	}
}

func max8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

// sortEntries orders the day's entries by day position, then by name.
func sortEntries(entries []PlanEntry) {
	sort.SliceStable(entries, func(x, y int) bool {
		if entries[x].Position != entries[y].Position {
			return entries[x].Position < entries[y].Position
		}
		return entries[x].DayName < entries[y].DayName
	})
}

// entriesFor collects the entries of all active user programs on date d.
func entriesFor(ups []UserProgram, d time.Time) []PlanEntry {
	entries := []PlanEntry{}
	for _, up := range ups {
		start, err := time.Parse("2006-01-02", up.StartDate)
		if err != nil {
			continue // corrupted anchor: skip the program, do not break the plan
		}
		start = dateOnly(start)
		for _, rule := range up.Rules {
			if !rule.IsEnabled || !ruleMatches(rule, start, dateOnly(d)) {
				continue
			}
			entries = append(entries, PlanEntry{
				UserProgramID: up.ID,
				ProgramID:     up.ProgramID,
				ProgramName:   up.ProgramName,
				ProgramDayID:  rule.ProgramDayID,
				DayName:       rule.DayName,
				Position:      rule.DayPosition,
			})
		}
	}
	sortEntries(entries)
	return entries
}

// Week returns the plan for the Monday-based week containing date
// (empty date = current week). The plan merges all active user programs.
func (s *Service) Week(ctx context.Context, userID uint32, date string) (*WeekPlan, error) {
	anchor := time.Now().UTC()
	if date != "" {
		var err error
		anchor, err = time.Parse("2006-01-02", date)
		if err != nil {
			return nil, ErrInvalidDate
		}
	}
	monday := mondayOf(anchor)

	ups, err := s.loadUserPrograms(ctx, userID)
	if err != nil {
		return nil, err
	}

	days := make([]PlanDay, 7)
	for i := range days {
		d := monday.AddDate(0, 0, i)
		days[i] = PlanDay{
			Date:    d.Format("2006-01-02"),
			Weekday: weekdayCodes[d.Weekday()],
			Entries: entriesFor(ups, d),
		}
	}

	return &WeekPlan{
		StartDate: monday.Format("2006-01-02"),
		EndDate:   monday.AddDate(0, 0, 6).Format("2006-01-02"),
		Days:      days,
	}, nil
}

// nextWindowDays limits how far ahead the next-workout lookup scans.
const nextWindowDays = 90

// NextWorkout is the closest scheduled workout on or after today.
// Date/Weekday are empty and DaysUntil is -1 when nothing is scheduled
// within the look-ahead window.
type NextWorkout struct {
	Date      string      `json:"date"`       // YYYY-MM-DD
	Weekday   string      `json:"weekday"`    // mon..sun
	DaysUntil int         `json:"days_until"` // 0 = today, -1 = none
	Entries   []PlanEntry `json:"entries"`
}

// Next returns the closest scheduled workout of the user's active programs.
// The scan starts at from (inclusive; YYYY-MM-DD; the client passes its
// local date so the server never guesses the user's "today") and covers
// nextWindowDays days. Empty from = UTC today.
func (s *Service) Next(ctx context.Context, userID uint32, from string) (*NextWorkout, error) {
	start := today()
	if from != "" {
		parsed, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, ErrInvalidDate
		}
		start = dateOnly(parsed)
	}

	ups, err := s.loadUserPrograms(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := 0; i < nextWindowDays; i++ {
		d := start.AddDate(0, 0, i)
		entries := entriesFor(ups, d)
		if len(entries) == 0 {
			continue
		}
		return &NextWorkout{
			Date:      d.Format("2006-01-02"),
			Weekday:   weekdayCodes[d.Weekday()],
			DaysUntil: i,
			Entries:   entries,
		}, nil
	}
	return &NextWorkout{Date: "", Weekday: "", DaysUntil: -1, Entries: []PlanEntry{}}, nil
}
