// Package workouts provides workout sessions: starting a workout from a
// program day (plan snapshot), tracking the fact per set, deviations
// (skip/replace/add) and completion (sqlx, raw SQL).
package workouts

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Service is the workouts repository and business logic in one place.
type Service struct {
	db  *sqlx.DB
	log *slog.Logger
}

func NewService(db *sqlx.DB, log *slog.Logger) *Service {
	return &Service{db: db, log: log}
}

// SessionSet is one set of a workout exercise: plan snapshot + fact.
// completed_at == nil means the set was not performed.
type SessionSet struct {
	ID                 uint32     `json:"id"`
	SetNumber          uint8      `json:"set_number"`
	PlannedReps        *uint16    `json:"planned_reps"`
	PlannedWeightKg    *float64   `json:"planned_weight_kg"`
	PlannedDurationSec *uint32    `json:"planned_duration_sec"`
	PlannedDistanceM   *uint32    `json:"planned_distance_m"`
	PlannedRestSec     *uint16    `json:"planned_rest_sec"`
	Reps               *uint16    `json:"reps"`
	WeightKg           *float64   `json:"weight_kg"`
	DurationSec        *uint32    `json:"duration_sec"`
	DistanceM          *uint32    `json:"distance_m"`
	CompletedAt        *time.Time `json:"completed_at"`
}

// SessionExercise is one catalog exercise inside a workout.
// ID is workout_exercises.id; replaced = the user swapped the catalog
// exercise of a planned one (see migration 00005).
type SessionExercise struct {
	ID                uint32       `json:"id"`
	ExerciseID        uint16       `json:"exercise_id"`
	ProgramExerciseID *uint32      `json:"program_exercise_id"`
	Replaced          bool         `json:"replaced"`
	Name              string       `json:"name"`
	Equipment         string       `json:"equipment"`
	TrackingType      string       `json:"tracking_type"`
	Position          uint8        `json:"position"`
	RestAfterSec      *uint16      `json:"rest_after_sec"`
	IsSkipped         bool         `json:"is_skipped"`
	ThumbnailURL      string       `json:"thumbnail_url"`
	GifURL            string       `json:"gif_url"`
	Sets              []SessionSet `json:"sets"`
}

// Session is the full state of one workout (started_at drives the
// client-side total duration timer).
type Session struct {
	ID            uint32            `json:"id"`
	UserProgramID *uint32           `json:"user_program_id"`
	ProgramDayID  *uint32           `json:"program_day_id"`
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	StartedAt     time.Time         `json:"started_at"`
	FinishedAt    *time.Time        `json:"finished_at"`
	Exercises     []SessionExercise `json:"exercises"`
}

const selectProgramDayQuery = `
	SELECT up.id AS user_program_id, up.program_id, pd.id AS day_id, pd.name AS day_name
	FROM user_programs up
	JOIN program_days pd ON pd.program_id = up.program_id
	WHERE up.id = ? AND pd.id = ? AND up.user_id = ? AND up.is_active = TRUE`

const selectDayExercisesQuery = `
	SELECT pe.id, pe.exercise_id, pe.position, pe.rest_after_sec
	FROM program_exercises pe
	WHERE pe.program_day_id = ?
	ORDER BY pe.position, pe.id`

const selectDaySetsQuery = `
	SELECT ps.program_exercise_id, ps.set_number, ps.reps, ps.weight_kg,
	       ps.duration_sec, ps.distance_m, ps.rest_sec
	FROM program_sets ps
	JOIN program_exercises pe ON pe.id = ps.program_exercise_id
	WHERE pe.program_day_id = ?
	ORDER BY ps.program_exercise_id, ps.set_number`

const insertWorkoutQuery = `
	INSERT INTO workouts (user_id, user_program_id, program_day_id, name)
	VALUES (?, ?, ?, ?)`

const insertWorkoutExerciseQuery = `
	INSERT INTO workout_exercises (workout_id, exercise_id, program_exercise_id, position, rest_after_sec)
	VALUES (?, ?, ?, ?, ?)`

const insertWorkoutSetQuery = `
	INSERT INTO workout_sets
	    (workout_exercise_id, set_number, planned_reps, planned_weight_kg,
	     planned_duration_sec, planned_distance_m, planned_rest_sec)
	VALUES (?, ?, ?, ?, ?, ?, ?)`

// selectActiveQuery finds the user's single unfinished workout.
const selectActiveQuery = `
	SELECT id, user_program_id, program_day_id, name, status, started_at, finished_at
	FROM workouts
	WHERE user_id = ? AND status = 'in_progress'
	LIMIT 1`

// selectSessionExercisesQueryTmpl loads the workout exercises with the
// catalog info the session screen needs (localized name, tracking type,
// gif/thumbnail for the "how to do it" visualization).
// %[1]s is the localized equipment column (see programs.nameColumn rule).
const selectSessionExercisesQueryTmpl = `
	SELECT
	    wex.id,
	    wex.exercise_id,
	    wex.program_exercise_id,
	    (wex.program_exercise_id IS NOT NULL AND pe.exercise_id <> wex.exercise_id) AS replaced,
	    wex.position,
	    wex.rest_after_sec,
	    wex.is_skipped,
	    t.name,
	    e.tracking_type,
	    eq.%[1]s AS equipment_name,
	    (SELECT g.path FROM exercise_media g
	     WHERE g.exercise_id = e.id AND g.type = 'gif'
	     ORDER BY g.sort_order, g.id LIMIT 1) AS gif_path,
	    (SELECT th.path FROM exercise_media th
	     WHERE th.exercise_id = e.id AND th.type = 'thumbnail'
	     ORDER BY th.sort_order, th.id LIMIT 1) AS thumbnail_path
	FROM workout_exercises wex
	JOIN exercises e ON e.id = wex.exercise_id
	JOIN exercise_translations t ON t.exercise_id = e.id AND t.lang = ?
	JOIN equipment eq ON eq.id = e.equipment_id
	LEFT JOIN program_exercises pe ON pe.id = wex.program_exercise_id
	WHERE wex.workout_id = ?
	ORDER BY wex.position, wex.id`

const selectSessionSetsQuery = `
	SELECT id, workout_exercise_id, set_number,
	       planned_reps, planned_weight_kg, planned_duration_sec,
	       planned_distance_m, planned_rest_sec,
	       reps, weight_kg, duration_sec, distance_m, completed_at
	FROM workout_sets
	WHERE workout_exercise_id IN (?)
	ORDER BY workout_exercise_id, set_number`

type programDayRow struct {
	UserProgramID uint32 `db:"user_program_id"`
	ProgramID     uint32 `db:"program_id"`
	DayID         uint32 `db:"day_id"`
	DayName       string `db:"day_name"`
}

type dayExerciseRow struct {
	ID           uint32  `db:"id"`
	ExerciseID   uint16  `db:"exercise_id"`
	Position     uint8   `db:"position"`
	RestAfterSec *uint16 `db:"rest_after_sec"`
}

type daySetRow struct {
	ProgramExerciseID uint32   `db:"program_exercise_id"`
	SetNumber         uint8    `db:"set_number"`
	Reps              *uint16  `db:"reps"`
	WeightKg          *float64 `db:"weight_kg"`
	DurationSec       *uint32  `db:"duration_sec"`
	DistanceM         *uint32  `db:"distance_m"`
	RestSec           *uint16  `db:"rest_sec"`
}

type workoutRow struct {
	ID            uint32     `db:"id"`
	UserProgramID *uint32    `db:"user_program_id"`
	ProgramDayID  *uint32    `db:"program_day_id"`
	Name          string     `db:"name"`
	Status        string     `db:"status"`
	StartedAt     time.Time  `db:"started_at"`
	FinishedAt    *time.Time `db:"finished_at"`
}

type sessionExerciseRow struct {
	ID                uint32  `db:"id"`
	ExerciseID        uint16  `db:"exercise_id"`
	ProgramExerciseID *uint32 `db:"program_exercise_id"`
	Replaced          bool    `db:"replaced"`
	Position          uint8   `db:"position"`
	RestAfterSec      *uint16 `db:"rest_after_sec"`
	IsSkipped         bool    `db:"is_skipped"`
	Name              string  `db:"name"`
	TrackingType      string  `db:"tracking_type"`
	EquipmentName     string  `db:"equipment_name"`
	GifPath           *string `db:"gif_path"`
	ThumbnailPath     *string `db:"thumbnail_path"`
}

type sessionSetRow struct {
	ID                 uint32     `db:"id"`
	WorkoutExerciseID  uint32     `db:"workout_exercise_id"`
	SetNumber          uint8      `db:"set_number"`
	PlannedReps        *uint16    `db:"planned_reps"`
	PlannedWeightKg    *float64   `db:"planned_weight_kg"`
	PlannedDurationSec *uint32    `db:"planned_duration_sec"`
	PlannedDistanceM   *uint32    `db:"planned_distance_m"`
	PlannedRestSec     *uint16    `db:"planned_rest_sec"`
	Reps               *uint16    `db:"reps"`
	WeightKg           *float64   `db:"weight_kg"`
	DurationSec        *uint32    `db:"duration_sec"`
	DistanceM          *uint32    `db:"distance_m"`
	CompletedAt        *time.Time `db:"completed_at"`
}

// isDuplicateActive reports the uq_workouts_one_active violation
// (concurrent start of a second workout).
func isDuplicateActive(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// Start creates a workout from a program day: the plan is snapshotted
// into planned_* columns so later program edits do not affect the
// running session. Returns the created session.
func (s *Service) Start(ctx context.Context, userID uint32, userProgramID uint32, programDayID uint32, lang string) (*Session, error) {
	var day programDayRow
	err := s.db.GetContext(ctx, &day, selectProgramDayQuery, userProgramID, programDayID, userID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var exRows []dayExerciseRow
	if err := s.db.SelectContext(ctx, &exRows, selectDayExercisesQuery, programDayID); err != nil {
		return nil, err
	}
	var setRows []daySetRow
	if len(exRows) > 0 {
		if err := s.db.SelectContext(ctx, &setRows, selectDaySetsQuery, programDayID); err != nil {
			return nil, err
		}
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, insertWorkoutQuery, userID, userProgramID, programDayID, day.DayName)
	if err != nil {
		if isDuplicateActive(err) {
			return nil, ErrActiveExists
		}
		return nil, err
	}
	workoutID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	setsByExercise := make(map[uint32][]daySetRow, len(exRows))
	for _, sr := range setRows {
		setsByExercise[sr.ProgramExerciseID] = append(setsByExercise[sr.ProgramExerciseID], sr)
	}

	for _, er := range exRows {
		res, err := tx.ExecContext(ctx, insertWorkoutExerciseQuery,
			workoutID, er.ExerciseID, er.ID, er.Position, er.RestAfterSec)
		if err != nil {
			return nil, err
		}
		wexID, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		for _, sr := range setsByExercise[er.ID] {
			if _, err := tx.ExecContext(ctx, insertWorkoutSetQuery,
				wexID, sr.SetNumber, sr.Reps, sr.WeightKg, sr.DurationSec, sr.DistanceM, sr.RestSec); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Active(ctx, userID, lang)
}

// Active returns the user's unfinished workout or nil when there is none.
// It is the recovery point after a page reload.
func (s *Service) Active(ctx context.Context, userID uint32, lang string) (*Session, error) {
	var w workoutRow
	err := s.db.GetContext(ctx, &w, selectActiveQuery, userID)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var exRows []sessionExerciseRow
	if err := s.db.SelectContext(ctx, &exRows, exerciseListQuery(lang), lang, w.ID); err != nil {
		return nil, err
	}

	session := &Session{
		ID:            w.ID,
		UserProgramID: w.UserProgramID,
		ProgramDayID:  w.ProgramDayID,
		Name:          w.Name,
		Status:        w.Status,
		StartedAt:     w.StartedAt,
		FinishedAt:    w.FinishedAt,
		Exercises:     make([]SessionExercise, 0, len(exRows)),
	}

	wexIDs := make([]uint32, 0, len(exRows))
	wexIndex := make(map[uint32]int, len(exRows))
	for i, er := range exRows {
		session.Exercises = append(session.Exercises, SessionExercise{
			ID:                er.ID,
			ExerciseID:        er.ExerciseID,
			ProgramExerciseID: er.ProgramExerciseID,
			Replaced:          er.Replaced,
			Name:              er.Name,
			Equipment:         er.EquipmentName,
			TrackingType:      er.TrackingType,
			Position:          er.Position,
			RestAfterSec:      er.RestAfterSec,
			IsSkipped:         er.IsSkipped,
			ThumbnailURL:      mediaURL(er.ThumbnailPath),
			GifURL:            mediaURL(er.GifPath),
			Sets:              []SessionSet{},
		})
		wexIndex[er.ID] = i
		wexIDs = append(wexIDs, er.ID)
	}
	if len(wexIDs) == 0 {
		return session, nil
	}

	query, args, err := sqlx.In(selectSessionSetsQuery, wexIDs)
	if err != nil {
		return nil, err
	}
	var setRows []sessionSetRow
	if err := s.db.SelectContext(ctx, &setRows, query, args...); err != nil {
		return nil, err
	}
	for _, sr := range setRows {
		idx, ok := wexIndex[sr.WorkoutExerciseID]
		if !ok {
			continue
		}
		session.Exercises[idx].Sets = append(session.Exercises[idx].Sets, SessionSet{
			ID:                 sr.ID,
			SetNumber:          sr.SetNumber,
			PlannedReps:        sr.PlannedReps,
			PlannedWeightKg:    sr.PlannedWeightKg,
			PlannedDurationSec: sr.PlannedDurationSec,
			PlannedDistanceM:   sr.PlannedDistanceM,
			PlannedRestSec:     sr.PlannedRestSec,
			Reps:               sr.Reps,
			WeightKg:           sr.WeightKg,
			DurationSec:        sr.DurationSec,
			DistanceM:          sr.DistanceM,
			CompletedAt:        sr.CompletedAt,
		})
	}
	return session, nil
}

// loadSessionExercise returns one exercise of the session with its sets
// (ownership enforced through workouts.user_id).
func (s *Service) loadSessionExercise(ctx context.Context, userID uint32, wexID uint32, lang string) (*SessionExercise, error) {
	var owner struct {
		WorkoutID uint32 `db:"workout_id"`
	}
	err := s.db.GetContext(ctx, &owner, selectExerciseOwnerQuery, wexID, userID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var exRows []sessionExerciseRow
	if err := s.db.SelectContext(ctx, &exRows, exerciseListQuery(lang), lang, owner.WorkoutID); err != nil {
		return nil, err
	}
	var target *sessionExerciseRow
	for i := range exRows {
		if exRows[i].ID == wexID {
			target = &exRows[i]
			break
		}
	}
	if target == nil {
		return nil, ErrNotFound
	}

	query, args, err := sqlx.In(selectSessionSetsQuery, wexID)
	if err != nil {
		return nil, err
	}
	var setRows []sessionSetRow
	if err := s.db.SelectContext(ctx, &setRows, query, args...); err != nil {
		return nil, err
	}

	out := &SessionExercise{
		ID:                target.ID,
		ExerciseID:        target.ExerciseID,
		ProgramExerciseID: target.ProgramExerciseID,
		Replaced:          target.Replaced,
		Name:              target.Name,
		Equipment:         target.EquipmentName,
		TrackingType:      target.TrackingType,
		Position:          target.Position,
		RestAfterSec:      target.RestAfterSec,
		IsSkipped:         target.IsSkipped,
		ThumbnailURL:      mediaURL(target.ThumbnailPath),
		GifURL:            mediaURL(target.GifPath),
		Sets:              make([]SessionSet, 0, len(setRows)),
	}
	for _, sr := range setRows {
		out.Sets = append(out.Sets, SessionSet{
			ID:                 sr.ID,
			SetNumber:          sr.SetNumber,
			PlannedReps:        sr.PlannedReps,
			PlannedWeightKg:    sr.PlannedWeightKg,
			PlannedDurationSec: sr.PlannedDurationSec,
			PlannedDistanceM:   sr.PlannedDistanceM,
			PlannedRestSec:     sr.PlannedRestSec,
			Reps:               sr.Reps,
			WeightKg:           sr.WeightKg,
			DurationSec:        sr.DurationSec,
			DistanceM:          sr.DistanceM,
			CompletedAt:        sr.CompletedAt,
		})
	}
	return out, nil
}
