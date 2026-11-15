package programs

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// Input is the create/update body: the program header plus the full
// structure (days -> exercises -> sets). Positions are derived from
// the slice order.
type Input struct {
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	AuthorName  *string    `json:"author_name"`
	Level       *string    `json:"level"`
	IsPublished bool       `json:"is_published"`
	Days        []DayInput `json:"days"`
}

type DayInput struct {
	Name      string          `json:"name"`
	Schedule  *ScheduleInput  `json:"schedule"`
	Exercises []ExerciseInput `json:"exercises"`
}

// ScheduleInput is the recurrence rule of a day:
// daily — every interval_n days; weekly — on weekdays every interval_n
// weeks; once — on the exact date. Nil means the day has no schedule.
type ScheduleInput struct {
	Frequency string   `json:"frequency"`
	IntervalN uint8    `json:"interval_n"`
	Weekdays  []string `json:"weekdays"`
	OnceDate  *string  `json:"once_date"` // YYYY-MM-DD, required for once
}

type ExerciseInput struct {
	ExerciseID   uint16     `json:"exercise_id"`
	RestAfterSec *uint16    `json:"rest_after_sec"`
	Sets         []SetInput `json:"sets"`
}

type SetInput struct {
	SetNumber   uint8    `json:"set_number"`
	Reps        *uint16  `json:"reps"`
	WeightKg    *float64 `json:"weight_kg"`
	DurationSec *uint32  `json:"duration_sec"`
	DistanceM   *uint32  `json:"distance_m"`
	RestSec     *uint16  `json:"rest_sec"`
}

// Limits guard against unreasonable payloads (schema columns are small).
const (
	MaxDays            = 50
	MaxExercisesPerDay = 50
	MaxSetsPerExercise = 50
	minWeightKg        = -300 // negative = assistance (assisted pull-ups)
	maxWeightKg        = 500
)

var (
	levels = map[string]struct{}{
		"beginner":     {},
		"intermediate": {},
		"advanced":     {},
	}
)

const (
	insertProgramQuery = `
		INSERT INTO programs (owner_id, copied_from_id, name, description, author_name, level, is_published)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	updateProgramQuery = `
		UPDATE programs
		SET name = ?, description = ?, author_name = ?, level = ?, is_published = ?
		WHERE id = ?`

	selectProgramForUpdateQuery = `
		SELECT id, owner_id, copied_from_id, name, description, author_name,
		       level, is_published
		FROM programs
		WHERE id = ?
		FOR UPDATE`

	deleteProgramQuery  = `DELETE FROM programs WHERE id = ?`
	deleteDaysQuery     = `DELETE FROM program_days WHERE program_id = ?`
	insertDayQuery      = `INSERT INTO program_days (program_id, position, name) VALUES (?, ?, ?)`
	insertScheduleQuery = `
		INSERT INTO program_schedule_rules (program_day_id, frequency, interval_n, weekdays, once_date)
		VALUES (?, ?, ?, ?, ?)`
	insertExerciseQuery = `INSERT INTO program_exercises (program_day_id, exercise_id, position, rest_after_sec) VALUES (?, ?, ?, ?)`
	insertSetQuery      = `
		INSERT INTO program_sets (program_exercise_id, set_number, reps, weight_kg, duration_sec, distance_m, rest_sec)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	checkExercisesQuery = `SELECT id FROM exercises WHERE id IN (?) AND is_active = 1`
)

// create converts Input into the internal insert shape, applying validation.
func (in *Input) toStructure() ([]dayInsert, error) {
	days := make([]dayInsert, 0, len(in.Days))
	for _, d := range in.Days {
		var schedule *scheduleInsert
		if d.Schedule != nil {
			s, err := d.Schedule.validate()
			if err != nil {
				return nil, err
			}
			schedule = &s
		}
		exs := make([]exInsert, 0, len(d.Exercises))
		for _, e := range d.Exercises {
			sets := make([]setInsert, 0, len(e.Sets))
			seen := make(map[uint8]struct{}, len(e.Sets))
			for _, st := range e.Sets {
				if st.SetNumber == 0 {
					return nil, ErrInvalidSetNumber
				}
				if _, dup := seen[st.SetNumber]; dup {
					return nil, ErrDuplicateSetNumber
				}
				seen[st.SetNumber] = struct{}{}
				if st.WeightKg != nil && (*st.WeightKg < minWeightKg || *st.WeightKg > maxWeightKg) {
					return nil, ErrInvalidWeight
				}
				sets = append(sets, setInsert{
					setNumber: st.SetNumber,
					reps:      st.Reps,
					weight:    st.WeightKg,
					duration:  st.DurationSec,
					distance:  st.DistanceM,
					rest:      st.RestSec,
				})
			}
			exs = append(exs, exInsert{
				exerciseID:   e.ExerciseID,
				restAfterSec: e.RestAfterSec,
				sets:         sets,
			})
		}
		days = append(days, dayInsert{name: trim(d.Name), schedule: schedule, exercises: exs})
	}
	return days, nil
}

// validate checks the header and the structural limits.
func (in *Input) validate() error {
	name := trim(in.Name)
	if name == "" || len([]rune(name)) > 255 {
		return ErrInvalidName
	}
	if in.Level != nil {
		if _, ok := levels[*in.Level]; !ok {
			return ErrInvalidLevel
		}
	}
	if len(in.Days) > MaxDays {
		return ErrTooManyItems
	}
	for _, d := range in.Days {
		if len(d.Exercises) > MaxExercisesPerDay {
			return ErrTooManyItems
		}
		if d.Schedule != nil {
			if _, err := d.Schedule.validate(); err != nil {
				return err
			}
		}
		for _, e := range d.Exercises {
			if len(e.Sets) > MaxSetsPerExercise {
				return ErrTooManyItems
			}
		}
	}
	return nil
}

var validWeekdays = map[string]struct{}{
	"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {}, "sat": {}, "sun": {},
}

// ValidatedSchedule is a normalized recurrence rule ready for storage.
type ValidatedSchedule struct {
	IntervalN uint8
	Weekdays  string     // canonical "mon,wed,fri"; empty for daily/once
	OnceDate  *time.Time // set for once
}

// NormalizeSchedule validates a recurrence rule and returns its storage
// shape; shared with the schedule domain (user rule edits).
func NormalizeSchedule(frequency string, intervalN uint8, weekdays []string, onceDate *string) (ValidatedSchedule, error) {
	out := ValidatedSchedule{IntervalN: intervalN}
	switch frequency {
	case "daily":
		if len(weekdays) != 0 || onceDate != nil || out.IntervalN == 0 {
			return out, ErrInvalidSchedule
		}
		return out, nil
	case "weekly":
		if onceDate != nil || len(weekdays) == 0 || len(weekdays) > len(weekdayOrder) || out.IntervalN == 0 {
			return out, ErrInvalidSchedule
		}
		set := make(map[string]struct{}, len(weekdays))
		for _, w := range weekdays {
			w = strings.ToLower(strings.TrimSpace(w))
			if _, ok := validWeekdays[w]; !ok {
				return out, ErrInvalidSchedule
			}
			if _, dup := set[w]; dup {
				return out, ErrInvalidSchedule
			}
			set[w] = struct{}{}
		}
		out.Weekdays = joinWeekdays(set)
		return out, nil
	case "once":
		if len(weekdays) != 0 || onceDate == nil {
			return out, ErrInvalidSchedule
		}
		date, err := time.Parse("2006-01-02", strings.TrimSpace(*onceDate))
		if err != nil {
			return out, ErrInvalidSchedule
		}
		out.IntervalN = 1
		out.OnceDate = &date
		return out, nil
	default:
		return out, ErrInvalidSchedule
	}
}

// validate checks the recurrence rule and returns it in the insert shape.
func (in *ScheduleInput) validate() (scheduleInsert, error) {
	v, err := NormalizeSchedule(in.Frequency, in.IntervalN, in.Weekdays, in.OnceDate)
	if err != nil {
		return scheduleInsert{}, err
	}
	return scheduleInsert{
		frequency: in.Frequency,
		intervalN: v.IntervalN,
		weekdays:  v.Weekdays,
		onceDate:  v.OnceDate,
	}, nil
}

func trim(s string) string {
	return strings.TrimSpace(s)
}

// ownership resolves owner_id/is_published for a create:
// an admin may publish a ready program (owner NULL), everything
// else is a personal program of the current user.
func ownership(userID uint32, isAdmin, isPublished bool) (*uint32, bool) {
	if isAdmin && isPublished {
		return nil, true
	}
	owner := userID
	return &owner, false
}

type setInsert struct {
	setNumber uint8
	reps      *uint16
	weight    *float64
	duration  *uint32
	distance  *uint32
	rest      *uint16
}

type exInsert struct {
	exerciseID   uint16
	restAfterSec *uint16
	sets         []setInsert
}

type dayInsert struct {
	name      string
	schedule  *scheduleInsert
	exercises []exInsert
}

// scheduleInsert is a validated recurrence rule ready for INSERT.
type scheduleInsert struct {
	frequency string
	intervalN uint8
	weekdays  string     // canonical "mon,wed,fri"; empty for daily/once
	onceDate  *time.Time // set for once
}

// values returns the driver args: empty weekdays must be stored as NULL
// (CHECK constraint), not as an empty SET string.
func (s *scheduleInsert) values() (uint8, any, any) {
	var weekdays any
	if s.weekdays != "" {
		weekdays = s.weekdays
	}
	var onceDate any
	if s.onceDate != nil {
		onceDate = s.onceDate
	}
	return s.intervalN, weekdays, onceDate
}

// Create stores a new program with the whole structure in one transaction
// and returns its id.
func (s *Service) Create(ctx context.Context, userID uint32, isAdmin bool, lang string, in *Input) (uint32, error) {
	if err := in.validate(); err != nil {
		return 0, err
	}
	days, err := in.toStructure()
	if err != nil {
		return 0, err
	}
	if err := checkDayNames(days); err != nil {
		return 0, err
	}

	ownerID, isPublished := ownership(userID, isAdmin, in.IsPublished)

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // safe: tx is either committed or already rolled back

	if err := checkExercisesExist(ctx, tx, days); err != nil {
		return 0, err
	}

	name := trim(in.Name)
	res, err := tx.ExecContext(ctx, insertProgramQuery,
		ownerID, nil, name, in.Description, in.AuthorName, in.Level, isPublished)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := insertStructure(ctx, tx, uint32(id), days); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint32(id), nil
}

// Update replaces the program header and structure in one transaction.
// Personal programs stay personal (is_published forced false);
// ready programs are editable by admins and may toggle is_published.
func (s *Service) Update(ctx context.Context, userID uint32, isAdmin bool, id uint32, in *Input) error {
	if err := in.validate(); err != nil {
		return err
	}
	days, err := in.toStructure()
	if err != nil {
		return err
	}
	if err := checkDayNames(days); err != nil {
		return err
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // safe: tx is either committed or already rolled back

	var row detailRow
	if err := tx.GetContext(ctx, &row, selectProgramForUpdateQuery, id); err != nil {
		if isNoRows(err) {
			return ErrNotFound
		}
		return err
	}
	if !row.canEdit(userID, isAdmin) {
		return ErrForbidden
	}

	isPublished := false
	if row.OwnerID == nil {
		isPublished = in.IsPublished
	}

	if _, err := tx.ExecContext(ctx, updateProgramQuery,
		trim(in.Name), in.Description, in.AuthorName, in.Level, isPublished, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, deleteDaysQuery, id); err != nil {
		return err
	}

	if err := checkExercisesExist(ctx, tx, days); err != nil {
		return err
	}
	if err := insertStructure(ctx, tx, id, days); err != nil {
		return err
	}
	return tx.Commit()
}

// Delete removes the program if the user may modify it.
// Days, exercises and sets are removed by cascade.
func (s *Service) Delete(ctx context.Context, userID uint32, isAdmin bool, id uint32) error {
	var row detailRow
	err := s.db.GetContext(ctx, &row, detailQuery, id)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !row.canEdit(userID, isAdmin) {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, deleteProgramQuery, id)
	return err
}

const copyDaysQuery = `
	SELECT id, position, name FROM program_days WHERE program_id = ? ORDER BY position, id`

const copySchedulesQuery = `
	SELECT program_day_id, frequency, interval_n, weekdays, once_date
	FROM program_schedule_rules
	WHERE program_day_id IN (?)`

const copyExercisesQuery = `
	SELECT id, exercise_id, program_day_id, position, rest_after_sec
	FROM program_exercises
	WHERE program_day_id IN (?)
	ORDER BY position, id`

const copySetsQuery = `
	SELECT program_exercise_id, set_number, reps, weight_kg, duration_sec, distance_m, rest_sec
	FROM program_sets
	WHERE program_exercise_id IN (?)
	ORDER BY set_number`

type copyExerciseRow struct {
	ID           uint32  `db:"id"`
	ExerciseID   uint16  `db:"exercise_id"`
	ProgramDayID uint32  `db:"program_day_id"`
	Position     uint8   `db:"position"`
	RestAfterSec *uint16 `db:"rest_after_sec"`
}

type copySetRow struct {
	ProgramExerciseID uint32   `db:"program_exercise_id"`
	SetNumber         uint8    `db:"set_number"`
	Reps              *uint16  `db:"reps"`
	WeightKg          *float64 `db:"weight_kg"`
	DurationSec       *uint32  `db:"duration_sec"`
	DistanceM         *uint32  `db:"distance_m"`
	RestSec           *uint16  `db:"rest_sec"`
}

// Copy duplicates a visible program as a personal one of the current user
// (copied_from_id points at the source).
func (s *Service) Copy(ctx context.Context, userID uint32, isAdmin bool, id uint32) (uint32, error) {
	var row detailRow
	err := s.db.GetContext(ctx, &row, detailQuery, id)
	if isNoRows(err) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if !row.visibleTo(userID, isAdmin) {
		return 0, ErrNotFound
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // safe: tx is either committed or already rolled back

	owner := userID
	res, err := tx.ExecContext(ctx, insertProgramQuery,
		owner, id, row.Name, row.Description, row.AuthorName, row.Level, false)
	if err != nil {
		return 0, err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	newProgramID := uint32(newID)

	var srcDays []dayRow
	if err := tx.SelectContext(ctx, &srcDays, copyDaysQuery, id); err != nil {
		return 0, err
	}

	// Recurrence rules travel with the days.
	scheduleByDay := make(map[uint32]scheduleInsert, len(srcDays))
	if len(srcDays) > 0 {
		srcDayIDs := make([]uint32, 0, len(srcDays))
		for _, sd := range srcDays {
			srcDayIDs = append(srcDayIDs, sd.ID)
		}
		schQuery, schArgs, err := sqlx.In(copySchedulesQuery, srcDayIDs)
		if err != nil {
			return 0, err
		}
		var srcSchedules []scheduleRow
		if err := tx.SelectContext(ctx, &srcSchedules, schQuery, schArgs...); err != nil {
			return 0, err
		}
		for _, sr := range srcSchedules {
			ins := scheduleInsert{
				frequency: sr.Frequency,
				intervalN: sr.IntervalN,
				weekdays:  sr.Weekdays.String,
			}
			if sr.OnceDate.Valid {
				d := sr.OnceDate.Time
				ins.onceDate = &d
			}
			scheduleByDay[sr.ProgramDayID] = ins
		}
	}

	dayMap := make(map[uint32]uint32, len(srcDays))
	for _, sd := range srcDays {
		r, err := tx.ExecContext(ctx, insertDayQuery, newProgramID, sd.Position, sd.Name)
		if err != nil {
			return 0, err
		}
		newDayID, err := r.LastInsertId()
		if err != nil {
			return 0, err
		}
		if rule, ok := scheduleByDay[sd.ID]; ok {
			interval, weekdays, onceDate := rule.values()
			if _, err := tx.ExecContext(ctx, insertScheduleQuery,
				newDayID, rule.frequency, interval, weekdays, onceDate); err != nil {
				return 0, err
			}
		}
		dayMap[sd.ID] = uint32(newDayID)
	}
	if len(dayMap) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return newProgramID, nil
	}

	srcDayIDs := make([]uint32, 0, len(srcDays))
	for _, sd := range srcDays {
		srcDayIDs = append(srcDayIDs, sd.ID)
	}
	exQuery, exArgs, err := sqlx.In(copyExercisesQuery, srcDayIDs)
	if err != nil {
		return 0, err
	}
	var srcExercises []copyExerciseRow
	if err := tx.SelectContext(ctx, &srcExercises, exQuery, exArgs...); err != nil {
		return 0, err
	}

	exMap := make(map[uint32]uint32, len(srcExercises))
	for _, se := range srcExercises {
		r, err := tx.ExecContext(ctx, insertExerciseQuery,
			dayMap[se.ProgramDayID], se.ExerciseID, se.Position, se.RestAfterSec)
		if err != nil {
			return 0, err
		}
		newExID, err := r.LastInsertId()
		if err != nil {
			return 0, err
		}
		exMap[se.ID] = uint32(newExID)
	}
	if len(exMap) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return newProgramID, nil
	}

	srcExIDs := make([]uint32, 0, len(srcExercises))
	for _, se := range srcExercises {
		srcExIDs = append(srcExIDs, se.ID)
	}
	setQ, setArgs, err := sqlx.In(copySetsQuery, srcExIDs)
	if err != nil {
		return 0, err
	}
	var srcSets []copySetRow
	if err := tx.SelectContext(ctx, &srcSets, setQ, setArgs...); err != nil {
		return 0, err
	}
	for _, ss := range srcSets {
		if _, err := tx.ExecContext(ctx, insertSetQuery,
			exMap[ss.ProgramExerciseID], ss.SetNumber, ss.Reps, ss.WeightKg,
			ss.DurationSec, ss.DistanceM, ss.RestSec); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newProgramID, nil
}

// insertStructure writes days -> schedules -> exercises -> sets preserving the slice order.
func insertStructure(ctx context.Context, tx *sqlx.Tx, programID uint32, days []dayInsert) error {
	for i, d := range days {
		res, err := tx.ExecContext(ctx, insertDayQuery, programID, i+1, d.name)
		if err != nil {
			return err
		}
		dayID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if d.schedule != nil {
			interval, weekdays, onceDate := d.schedule.values()
			if _, err := tx.ExecContext(ctx, insertScheduleQuery,
				dayID, d.schedule.frequency, interval, weekdays, onceDate); err != nil {
				return err
			}
		}
		for j, e := range d.exercises {
			res, err := tx.ExecContext(ctx, insertExerciseQuery, dayID, e.exerciseID, j+1, e.restAfterSec)
			if err != nil {
				return err
			}
			exID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			for _, st := range e.sets {
				if _, err := tx.ExecContext(ctx, insertSetQuery,
					exID, st.setNumber, st.reps, st.weight, st.duration, st.distance, st.rest); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkExercisesExist validates every exercise id against the active catalog.
func checkExercisesExist(ctx context.Context, tx *sqlx.Tx, days []dayInsert) error {
	ids := make([]uint32, 0)
	seen := make(map[uint16]struct{})
	for _, d := range days {
		for _, e := range d.exercises {
			if _, ok := seen[e.exerciseID]; !ok {
				seen[e.exerciseID] = struct{}{}
				ids = append(ids, uint32(e.exerciseID))
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(checkExercisesQuery, ids)
	if err != nil {
		return err
	}
	var found []uint32
	if err := tx.SelectContext(ctx, &found, query, args...); err != nil {
		return err
	}
	if len(found) != len(ids) {
		return ErrUnknownExercise
	}
	return nil
}

// checkDayNames rejects empty/too long day names (the column is NOT NULL).
func checkDayNames(days []dayInsert) error {
	for _, d := range days {
		name := trim(d.name)
		if name == "" || len([]rune(name)) > 255 {
			return ErrInvalidDayName
		}
	}
	return nil
}
