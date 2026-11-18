// Package stats aggregates workout statistics for the dashboard:
// monthly counters, tonnage, schedule streak, weekly activity chart
// and the last completed workout (sqlx, raw SQL over stats views).
package stats

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/gladart32/gym/internal/schedule"
)

// streakWindowDays limits how far back the streak scan goes.
const streakWindowDays = 90

// activityWeeks is the number of past weeks returned for the chart
// including the current one.
const activityWeeks = 12

// Service is the stats repository and business logic in one place.
type Service struct {
	db       *sqlx.DB
	log      *slog.Logger
	schedule *schedule.Service
}

func NewService(db *sqlx.DB, log *slog.Logger, schedule *schedule.Service) *Service {
	return &Service{db: db, log: log, schedule: schedule}
}

// Streak is the schedule-based streak: scheduled slots of the active
// programs in the scan window vs closed (completed) ones.
type Streak struct {
	Current   int `json:"current"`   // closed slots in a row before the first miss
	Scheduled int `json:"scheduled"` // slots in the window (past dates only)
	Completed int `json:"completed"` // closed slots in the window
}

// WeekActivity is one ISO week (Monday-based) of the activity chart.
type WeekActivity struct {
	WeekStart     string  `json:"week_start"` // Monday, YYYY-MM-DD
	Workouts      int     `json:"workouts"`
	SetsDone      int     `json:"sets_done"`
	TotalVolumeKg float64 `json:"total_volume_kg"`
}

// LastWorkout is a short summary of the most recent completed workout.
type LastWorkout struct {
	Date          string  `json:"date"` // YYYY-MM-DD
	Name          string  `json:"name"`
	DurationSec   int     `json:"duration_sec"`
	SetsDone      int     `json:"sets_done"`
	ExercisesDone int     `json:"exercises_done"`
	TotalVolumeKg float64 `json:"total_volume_kg"`
}

// Dashboard is the whole payload for the main page in one response.
type Dashboard struct {
	WorkoutsMonth  int            `json:"workouts_month"`
	WorkoutsTotal  int            `json:"workouts_total"`
	TonnageMonthKg float64        `json:"tonnage_month_kg"`
	Streak         *Streak        `json:"streak"` // null = no active programs
	WeeklyActivity []WeekActivity `json:"weekly_activity"`
	LastWorkout    *LastWorkout   `json:"last_workout"` // null = none yet
}

const countCompletedQuery = `
	SELECT COUNT(*) FROM workouts
	WHERE user_id = ? AND status = 'completed' AND started_at >= ?`

const tonnageMonthQuery = `
	SELECT COALESCE(SUM(volume_kg), 0) FROM v_done_sets
	WHERE user_id = ? AND started_at >= ?`

const activityRowsQuery = `
	SELECT started_at, workout_id, volume_kg FROM v_done_sets
	WHERE user_id = ? AND started_at >= ?`

const lastWorkoutQuery = `
	SELECT w.name, DATE(w.started_at) AS workout_date,
	       COALESCE(s.duration_sec, 0)    AS duration_sec,
	       COALESCE(s.sets_done, 0)       AS sets_done,
	       COALESCE(s.exercises_done, 0)  AS exercises_done,
	       COALESCE(s.total_volume_kg, 0) AS total_volume_kg
	FROM workouts w
	LEFT JOIN v_workout_summary s ON s.workout_id = w.id
	WHERE w.user_id = ? AND w.status = 'completed'
	ORDER BY w.started_at DESC
	LIMIT 1`

const completedWorkoutsQuery = `
	SELECT user_program_id, program_day_id, started_at
	FROM workouts
	WHERE user_id = ? AND status = 'completed' AND started_at >= ?
	  AND user_program_id IS NOT NULL AND program_day_id IS NOT NULL`

const selectTimezoneQuery = `SELECT timezone FROM users WHERE id = ?`

// epochStart is a safe lower bound for "all time" queries
// (MySQL DATETIME starts at 1000-01-01).
var epochStart = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type lastWorkoutRow struct {
	Name          string    `db:"name"`
	WorkoutDate   time.Time `db:"workout_date"`
	DurationSec   int       `db:"duration_sec"`
	SetsDone      int       `db:"sets_done"`
	ExercisesDone int       `db:"exercises_done"`
	TotalVolumeKg float64   `db:"total_volume_kg"`
}

type completedWorkoutRow struct {
	UserProgramID uint32    `db:"user_program_id"`
	ProgramDayID  uint32    `db:"program_day_id"`
	StartedAt     time.Time `db:"started_at"`
}

type activityRow struct {
	StartedAt time.Time `db:"started_at"`
	WorkoutID uint64    `db:"workout_id"`
	VolumeKg  *float64  `db:"volume_kg"` // NULL for time/distance sets
}

// Dashboard returns all dashboard statistics for the user.
func (s *Service) Dashboard(ctx context.Context, userID uint32) (*Dashboard, error) {
	loc, err := s.userLocation(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	out := &Dashboard{WeeklyActivity: []WeekActivity{}}

	if err := s.db.GetContext(ctx, &out.WorkoutsMonth, countCompletedQuery, userID, monthStart); err != nil {
		return nil, err
	}
	if err := s.db.GetContext(ctx, &out.WorkoutsTotal, countCompletedQuery, userID, epochStart); err != nil {
		return nil, err
	}
	if err := s.db.GetContext(ctx, &out.TonnageMonthKg, tonnageMonthQuery, userID, monthStart); err != nil {
		return nil, err
	}

	if out.LastWorkout, err = s.lastWorkout(ctx, userID); err != nil {
		return nil, err
	}

	monday := mondayOf(now)
	if out.WeeklyActivity, err = s.weeklyActivity(ctx, userID, loc, monday); err != nil {
		return nil, err
	}

	if out.Streak, err = s.streak(ctx, userID, loc, now); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Service) lastWorkout(ctx context.Context, userID uint32) (*LastWorkout, error) {
	var row lastWorkoutRow
	err := s.db.GetContext(ctx, &row, lastWorkoutQuery, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // no completed workouts yet
	}
	if err != nil {
		return nil, err
	}
	return &LastWorkout{
		Date:          row.WorkoutDate.Format("2006-01-02"),
		Name:          row.Name,
		DurationSec:   row.DurationSec,
		SetsDone:      row.SetsDone,
		ExercisesDone: row.ExercisesDone,
		TotalVolumeKg: row.TotalVolumeKg,
	}, nil
}

// weeklyActivity buckets the done sets of the last activityWeeks weeks
// by the workout's local (user timezone) date, filling empty weeks with zeros.
func (s *Service) weeklyActivity(ctx context.Context, userID uint32, loc *time.Location, currentMonday time.Time) ([]WeekActivity, error) {
	start := currentMonday.AddDate(0, 0, -7*(activityWeeks-1))

	var rows []activityRow
	if err := s.db.SelectContext(ctx, &rows, activityRowsQuery, userID, start); err != nil {
		return nil, err
	}

	weeks := make([]WeekActivity, activityWeeks)
	seen := make([]map[uint64]struct{}, activityWeeks) // distinct workouts per week
	for i := range weeks {
		weeks[i] = WeekActivity{WeekStart: start.AddDate(0, 0, 7*i).Format("2006-01-02")}
		seen[i] = map[uint64]struct{}{}
	}

	for _, r := range rows {
		idx := daysBetween(start, r.StartedAt.In(loc)) / 7
		if idx < 0 || idx >= activityWeeks {
			continue
		}
		weeks[idx].SetsDone++
		if r.VolumeKg != nil {
			weeks[idx].TotalVolumeKg += *r.VolumeKg
		}
		if _, ok := seen[idx][r.WorkoutID]; !ok {
			seen[idx][r.WorkoutID] = struct{}{}
			weeks[idx].Workouts++
		}
	}
	return weeks, nil
}

// streak compares the scheduled occurrences of the active programs with
// the completed workouts (same program day, same local date). Today's and
// future slots never break the streak. No active programs -> nil.
func (s *Service) streak(ctx context.Context, userID uint32, loc *time.Location, now time.Time) (*Streak, error) {
	ups, err := s.schedule.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(ups) == 0 {
		return nil, nil
	}

	today := dateOnly(now)
	windowStart := today.AddDate(0, 0, -streakWindowDays)

	// Only slots strictly before today can be missed.
	slots, err := s.schedule.Occurrences(ctx, userID, windowStart, today.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}

	var cwRows []completedWorkoutRow
	if err := s.db.SelectContext(ctx, &cwRows, completedWorkoutsQuery, userID, windowStart); err != nil {
		return nil, err
	}
	// key "date:user_program_id:program_day_id" -> unconsumed workouts
	done := make(map[string][]completedWorkoutRow, len(cwRows))
	for _, r := range cwRows {
		key := workoutKey(r.UserProgramID, r.ProgramDayID, dateOnly(r.StartedAt.In(loc)).Format("2006-01-02"))
		done[key] = append(done[key], r)
	}

	// Oldest first: consume matching workouts slot by slot.
	sorted := make([]schedule.Occurrence, len(slots))
	copy(sorted, slots)
	sort.SliceStable(sorted, func(x, y int) bool { return sorted[x].Date < sorted[y].Date })

	st := &Streak{Scheduled: len(sorted)}
	closed := make([]bool, len(sorted))
	for i, slot := range sorted {
		key := workoutKey(slot.UserProgramID, slot.ProgramDayID, slot.Date)
		if list := done[key]; len(list) > 0 {
			done[key] = list[1:]
			closed[i] = true
			st.Completed++
		}
	}

	// Current streak: closed slots in a row up to the most recent miss.
	for i := len(sorted) - 1; i >= 0 && closed[i]; i-- {
		st.Current++
	}
	return st, nil
}

func workoutKey(userProgramID, programDayID uint32, date string) string {
	return date + ":" + strconv.FormatUint(uint64(userProgramID), 10) + ":" +
		strconv.FormatUint(uint64(programDayID), 10)
}

func (s *Service) userLocation(ctx context.Context, userID uint32) (*time.Location, error) {
	var tz string
	if err := s.db.GetContext(ctx, &tz, selectTimezoneQuery, userID); err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		return time.UTC, nil
	}
	return loc, nil
}

// dateOnly truncates a time to the UTC midnight of its date.
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// daysBetween returns the floored whole-day difference d - start.
func daysBetween(start, d time.Time) int {
	return int(dateOnly(d).Sub(dateOnly(start)).Hours() / 24)
}

// mondayOf returns the Monday of the week containing t (Monday-based week).
func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Mon -> 0 .. Sun -> 6
	return dateOnly(t).AddDate(0, 0, -offset)
}
