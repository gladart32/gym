package schedule

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gladart32/gym/internal/programs"
)

// RuleInput is the editable subset of a user recurrence rule.
type RuleInput struct {
	Frequency string   `json:"frequency"`
	IntervalN uint8    `json:"interval_n"`
	Weekdays  []string `json:"weekdays"`
	OnceDate  *string  `json:"once_date"`
	IsEnabled *bool    `json:"is_enabled"`
}

const insertUserProgramQuery = `
	INSERT INTO user_programs (user_id, program_id, start_date, is_active)
	VALUES (?, ?, ?, TRUE)`

// copyScheduleRulesQuery copies the program's default rules into personal
// ones, snapshotting the day name/position: later program edits must not
// affect started programs (no FK to program_days).
const copyScheduleRulesQuery = `
	INSERT INTO user_schedule_rules
	    (user_program_id, program_day_id, day_name, day_position, frequency, interval_n, weekdays, once_date, is_enabled)
	SELECT ?, psr.program_day_id, d.name, d.position, psr.frequency, psr.interval_n, psr.weekdays, psr.once_date, TRUE
	FROM program_schedule_rules psr
	JOIN program_days d ON d.id = psr.program_day_id
	WHERE d.program_id = ?`

const updateRuleQuery = `
	UPDATE user_schedule_rules
	SET frequency = ?, interval_n = ?, weekdays = ?, once_date = ?, is_enabled = ?
	WHERE id = ?`

const deleteRuleQuery = `DELETE FROM user_schedule_rules WHERE id = ?`

const deactivateUserProgramQuery = `
	UPDATE user_programs SET is_active = FALSE WHERE id = ?`

// Select activates a visible program for the user: creates a user_programs
// row and copies the program's default schedule rules into personal ones.
// Multiple active programs are allowed.
func (s *Service) Select(ctx context.Context, userID, programID uint32, isAdmin bool, startDate *string) (*UserProgram, error) {
	var prog struct {
		OwnerID     *uint32 `db:"owner_id"`
		IsPublished bool    `db:"is_published"`
	}
	err := s.db.GetContext(ctx, &prog, selectProgramVisibilityQuery, programID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	visible := (prog.OwnerID != nil && *prog.OwnerID == userID) ||
		(prog.OwnerID == nil && (prog.IsPublished || isAdmin))
	if !visible {
		return nil, ErrNotFound
	}

	var start time.Time
	if startDate == nil || *startDate == "" {
		start = today()
	} else {
		start, err = time.Parse("2006-01-02", *startDate)
		if err != nil {
			return nil, ErrInvalidStartDate
		}
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // safe: tx is either committed or already rolled back

	res, err := tx.ExecContext(ctx, insertUserProgramQuery, userID, programID, start)
	if err != nil {
		return nil, err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, copyScheduleRulesQuery, newID, programID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	ups, err := s.loadUserPrograms(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range ups {
		if ups[i].ID == uint32(newID) {
			return &ups[i], nil
		}
	}
	return nil, ErrNotFound
}

// Get returns the user's active programs with their rules.
func (s *Service) Get(ctx context.Context, userID uint32) ([]UserProgram, error) {
	return s.loadUserPrograms(ctx, userID)
}

// UpdateRule edits one user rule (frequency, interval, weekdays, once date,
// enabled flag). Access is limited to the owner of the user program.
func (s *Service) UpdateRule(ctx context.Context, userID, ruleID uint32, in *RuleInput) (*Rule, error) {
	var owner uint32
	err := s.db.GetContext(ctx, &owner, selectRuleOwnerQuery, ruleID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if owner != userID {
		return nil, ErrForbidden
	}

	norm, err := programs.NormalizeSchedule(in.Frequency, in.IntervalN, in.Weekdays, in.OnceDate)
	if err != nil {
		return nil, err
	}

	enabled := true
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}

	var weekdays, onceDate any
	if norm.Weekdays != "" {
		weekdays = norm.Weekdays
	}
	if norm.OnceDate != nil {
		onceDate = norm.OnceDate
	}
	if _, err := s.db.ExecContext(ctx, updateRuleQuery,
		in.Frequency, norm.IntervalN, weekdays, onceDate, enabled, ruleID); err != nil {
		return nil, err
	}

	// Re-read with day names/positions for the response.
	var rr ruleRow
	if err := s.db.GetContext(ctx, &rr, selectRuleByIDQuery, ruleID); err != nil {
		return nil, err
	}
	rule := rr.toRule()
	return &rule, nil
}

const selectRuleByIDQuery = `
	SELECT r.id, r.user_program_id, r.program_day_id, r.frequency, r.interval_n,
	       r.weekdays, r.once_date, r.is_enabled, r.day_name, r.day_position
	FROM user_schedule_rules r
	WHERE r.id = ?`

// DeleteRule removes one personal rule; access is limited to the owner
// of the user program. The rule disappears from the schedule and the
// week plan.
func (s *Service) DeleteRule(ctx context.Context, userID, ruleID uint32) error {
	var owner uint32
	err := s.db.GetContext(ctx, &owner, selectRuleOwnerQuery, ruleID)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != userID {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, deleteRuleQuery, ruleID)
	return err
}

// Deactivate disables a user program: it disappears from the schedule,
// the row and rules are kept for history.
func (s *Service) Deactivate(ctx context.Context, userID, userProgramID uint32) error {
	var owner uint32
	err := s.db.GetContext(ctx, &owner, selectUserProgramOwnerQuery, userProgramID)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != userID {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, deactivateUserProgramQuery, userProgramID)
	return err
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
