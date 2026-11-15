// Package schedule provides the user's selected programs (user_programs),
// their personal recurrence rules (user_schedule_rules) and the week plan
// computed from them (sqlx, raw SQL).
package schedule

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
)

// Service is the schedule repository and business logic in one place.
type Service struct {
	db  *sqlx.DB
	log *slog.Logger
}

func NewService(db *sqlx.DB, log *slog.Logger) *Service {
	return &Service{db: db, log: log}
}

// Rule is a personal recurrence rule of one program day.
type Rule struct {
	ID            uint32   `json:"id"`
	UserProgramID uint32   `json:"user_program_id"`
	ProgramDayID  uint32   `json:"program_day_id"`
	DayName       string   `json:"day_name"`
	DayPosition   uint8    `json:"day_position"`
	Frequency     string   `json:"frequency"`
	IntervalN     uint8    `json:"interval_n"`
	Weekdays      []string `json:"weekdays"`
	OnceDate      *string  `json:"once_date"`
	IsEnabled     bool     `json:"is_enabled"`
}

// UserProgram is a program selected by the user together with its rules.
type UserProgram struct {
	ID          uint32 `json:"id"`
	ProgramID   uint32 `json:"program_id"`
	ProgramName string `json:"program_name"`
	StartDate   string `json:"start_date"`
	IsActive    bool   `json:"is_active"`
	Rules       []Rule `json:"rules"`
}

// PlanEntry is one scheduled workout-day occurrence on a concrete date.
type PlanEntry struct {
	UserProgramID uint32 `json:"user_program_id"`
	ProgramID     uint32 `json:"program_id"`
	ProgramName   string `json:"program_name"`
	ProgramDayID  uint32 `json:"program_day_id"`
	DayName       string `json:"day_name"`
	Position      uint8  `json:"position"`
}

// PlanDay is a single calendar date with its scheduled entries.
type PlanDay struct {
	Date    string      `json:"date"` // YYYY-MM-DD
	Weekday string      `json:"weekday"`
	Entries []PlanEntry `json:"entries"`
}

// WeekPlan covers Monday..Sunday of one week.
type WeekPlan struct {
	StartDate string    `json:"start_date"` // Monday, YYYY-MM-DD
	EndDate   string    `json:"end_date"`   // Sunday, YYYY-MM-DD
	Days      []PlanDay `json:"days"`
}

const selectProgramVisibilityQuery = `
	SELECT owner_id, is_published
	FROM programs
	WHERE id = ?`

const selectUserProgramsQuery = `
	SELECT up.id, up.program_id, up.start_date, p.name AS program_name
	FROM user_programs up
	JOIN programs p ON p.id = up.program_id
	WHERE up.user_id = ? AND up.is_active = TRUE
	ORDER BY up.id`

const selectRulesQuery = `
	SELECT r.id, r.user_program_id, r.program_day_id, r.frequency, r.interval_n,
	       r.weekdays, r.once_date, r.is_enabled,
	       d.name AS day_name, d.position AS day_position
	FROM user_schedule_rules r
	JOIN program_days d ON d.id = r.program_day_id
	WHERE r.user_program_id IN (?)
	ORDER BY r.user_program_id, d.position, r.id`

const selectRuleOwnerQuery = `
	SELECT up.user_id
	FROM user_schedule_rules r
	JOIN user_programs up ON up.id = r.user_program_id
	WHERE r.id = ?`

const selectUserProgramOwnerQuery = `
	SELECT user_id FROM user_programs WHERE id = ?`

type userProgramRow struct {
	ID          uint32    `db:"id"`
	ProgramID   uint32    `db:"program_id"`
	StartDate   time.Time `db:"start_date"`
	ProgramName string    `db:"program_name"`
}

type ruleRow struct {
	ID            uint32         `db:"id"`
	UserProgramID uint32         `db:"user_program_id"`
	ProgramDayID  uint32         `db:"program_day_id"`
	Frequency     string         `db:"frequency"`
	IntervalN     uint8          `db:"interval_n"`
	Weekdays      sql.NullString `db:"weekdays"` // MySQL SET: "mon,wed,fri", NULL for daily/once
	OnceDate      sql.NullTime   `db:"once_date"`
	IsEnabled     bool           `db:"is_enabled"`
	DayName       string         `db:"day_name"`
	DayPosition   uint8          `db:"day_position"`
}

func (r ruleRow) toRule() Rule {
	rule := Rule{
		ID:            r.ID,
		UserProgramID: r.UserProgramID,
		ProgramDayID:  r.ProgramDayID,
		DayName:       r.DayName,
		DayPosition:   r.DayPosition,
		Frequency:     r.Frequency,
		IntervalN:     r.IntervalN,
		IsEnabled:     r.IsEnabled,
		Weekdays:      []string{},
	}
	if r.Weekdays.Valid {
		rule.Weekdays = splitWeekdays(r.Weekdays.String)
	}
	if r.OnceDate.Valid {
		date := r.OnceDate.Time.Format("2006-01-02")
		rule.OnceDate = &date
	}
	return rule
}

// loadUserPrograms returns the user's active programs with their rules.
func (s *Service) loadUserPrograms(ctx context.Context, userID uint32) ([]UserProgram, error) {
	var upRows []userProgramRow
	if err := s.db.SelectContext(ctx, &upRows, selectUserProgramsQuery, userID); err != nil {
		return nil, err
	}

	ups := make([]UserProgram, 0, len(upRows))
	upIndex := make(map[uint32]int, len(upRows))
	upIDs := make([]uint32, 0, len(upRows))
	for i, row := range upRows {
		upIDs = append(upIDs, row.ID)
		upIndex[row.ID] = i
		ups = append(ups, UserProgram{
			ID:          row.ID,
			ProgramID:   row.ProgramID,
			ProgramName: row.ProgramName,
			StartDate:   row.StartDate.Format("2006-01-02"),
			IsActive:    true,
			Rules:       []Rule{},
		})
	}
	if len(upIDs) == 0 {
		return ups, nil
	}

	query, args, err := sqlx.In(selectRulesQuery, upIDs)
	if err != nil {
		return nil, err
	}
	var rules []ruleRow
	if err := s.db.SelectContext(ctx, &rules, query, args...); err != nil {
		return nil, err
	}
	for _, rr := range rules {
		idx, ok := upIndex[rr.UserProgramID]
		if !ok {
			continue
		}
		ups[idx].Rules = append(ups[idx].Rules, rr.toRule())
	}
	return ups, nil
}
