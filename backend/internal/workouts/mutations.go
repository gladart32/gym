package workouts

import (
	"context"
	"time"
)

// Limits guard against unreasonable payloads (same spirit as programs).
const (
	MaxSetsPerExercise = 50
	MaxExercises       = 50
	minWeightKg        = -300 // negative = assistance (assisted pull-ups)
	maxWeightKg        = 500
	maxReps            = 2000
	maxDurationSec     = 86400 // 24h per set
	maxDistanceM       = 1000000
	maxRestSec         = 3600
)

// SetInput replaces the fact of one set (all fact fields are sent each
// time; omitted/null means the stored value is cleared).
// Completed: nil = unchanged, true = completed now, false = not performed.
type SetInput struct {
	Reps        *uint16  `json:"reps"`
	WeightKg    *float64 `json:"weight_kg"`
	DurationSec *uint32  `json:"duration_sec"`
	DistanceM   *uint32  `json:"distance_m"`
	Completed   *bool    `json:"completed"`
}

// ExerciseInput patches a workout exercise: skip/unskip and/or replace
// the catalog exercise.
type ExerciseInput struct {
	IsSkipped  *bool   `json:"is_skipped"`
	ExerciseID *uint16 `json:"exercise_id"`
}

// AddExerciseInput adds an unplanned exercise to a running workout;
// it starts with one empty set.
type AddExerciseInput struct {
	ExerciseID   uint16  `json:"exercise_id"`
	RestAfterSec *uint16 `json:"rest_after_sec"`
}

const (
	selectSetOwnerStatusQuery = `
		SELECT ws.completed_at, w.status
		FROM workout_sets ws
		JOIN workout_exercises wex ON wex.id = ws.workout_exercise_id
		JOIN workouts w ON w.id = wex.workout_id
		WHERE ws.id = ? AND w.user_id = ?`

	updateSetQuery = `
		UPDATE workout_sets
		SET reps = ?, weight_kg = ?, duration_sec = ?, distance_m = ?, completed_at = ?
		WHERE id = ?`

	// active-workout guards: ownership + status in one WHERE.
	selectActiveWorkoutIDQuery = `
		SELECT id FROM workouts
		WHERE user_id = ? AND status = 'in_progress'
		LIMIT 1`

	selectWorkoutExerciseActiveQuery = `
		SELECT wex.workout_id
		FROM workout_exercises wex
		JOIN workouts w ON w.id = wex.workout_id
		WHERE wex.id = ? AND w.user_id = ? AND w.status = 'in_progress'`

	nextSetNumberQuery = `
		SELECT COALESCE(MAX(set_number), 0) + 1
		FROM workout_sets
		WHERE workout_exercise_id = ?`

	nextPositionQuery = `
		SELECT COALESCE(MAX(position), 0) + 1
		FROM workout_exercises
		WHERE workout_id = ?`

	checkCatalogExerciseQuery = `SELECT id FROM exercises WHERE id = ? AND is_active = 1`

	insertWorkoutSetFactQuery = `
		INSERT INTO workout_sets (workout_exercise_id, set_number) VALUES (?, ?)`

	insertAddedExerciseQuery = `
		INSERT INTO workout_exercises (workout_id, exercise_id, position, rest_after_sec)
		VALUES (?, ?, ?, ?)`

	finishWorkoutQuery = `
		UPDATE workouts
		SET status = ?, finished_at = NOW()
		WHERE user_id = ? AND status = 'in_progress'`
)

// validate checks the fact values against schema-friendly ranges.
func (in *SetInput) validate() error {
	if in.Reps != nil && *in.Reps > maxReps {
		return ErrInvalidValue
	}
	if in.WeightKg != nil && (*in.WeightKg < minWeightKg || *in.WeightKg > maxWeightKg) {
		return ErrInvalidValue
	}
	if in.DurationSec != nil && *in.DurationSec > maxDurationSec {
		return ErrInvalidValue
	}
	if in.DistanceM != nil && *in.DistanceM > maxDistanceM {
		return ErrInvalidValue
	}
	return nil
}

// UpdateSet replaces the fact of one set. Allowed at any workout status
// (the fact can be corrected after completion, see migration 00005).
func (s *Service) UpdateSet(ctx context.Context, userID uint32, setID uint32, in *SetInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	var owner struct {
		CompletedAt *time.Time `db:"completed_at"`
		Status      string     `db:"status"`
	}
	err := s.db.GetContext(ctx, &owner, selectSetOwnerStatusQuery, setID, userID)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	completedAt := owner.CompletedAt
	if in.Completed != nil {
		if *in.Completed {
			now := time.Now().UTC()
			completedAt = &now
		} else {
			completedAt = nil
		}
	}

	_, err = s.db.ExecContext(ctx, updateSetQuery,
		in.Reps, in.WeightKg, in.DurationSec, in.DistanceM, completedAt, setID)
	return err
}

// AddSet appends an unplanned set to a workout exercise of a running workout.
func (s *Service) AddSet(ctx context.Context, userID uint32, wexID uint32) (*SessionSet, error) {
	if _, err := s.activeWorkoutExercise(ctx, userID, wexID); err != nil {
		return nil, err
	}

	var next uint8
	if err := s.db.GetContext(ctx, &next, nextSetNumberQuery, wexID); err != nil {
		return nil, err
	}
	if int(next) > MaxSetsPerExercise {
		return nil, ErrTooManySets
	}

	res, err := s.db.ExecContext(ctx, insertWorkoutSetFactQuery, wexID, next)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &SessionSet{ID: uint32(id), SetNumber: next}, nil
}

// UpdateExercise skips/unskips and/or replaces a workout exercise
// of a running workout.
func (s *Service) UpdateExercise(ctx context.Context, userID uint32, wexID uint32, lang string, in *ExerciseInput) (*SessionExercise, error) {
	if _, err := s.activeWorkoutExercise(ctx, userID, wexID); err != nil {
		return nil, err
	}

	set := ""
	args := []any{}
	if in.IsSkipped != nil {
		set += "is_skipped = ?, "
		args = append(args, *in.IsSkipped)
	}
	if in.ExerciseID != nil {
		if err := s.checkCatalogExercise(ctx, *in.ExerciseID); err != nil {
			return nil, err
		}
		set += "exercise_id = ?, "
		args = append(args, *in.ExerciseID)
	}
	if len(args) == 0 {
		// nothing to change — return the current state
		return s.loadSessionExercise(ctx, userID, wexID, lang)
	}
	query := "UPDATE workout_exercises SET " + set[:len(set)-2] + " WHERE id = ?"
	args = append(args, wexID)

	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	return s.loadSessionExercise(ctx, userID, wexID, lang)
}

// AddExercise appends an unplanned exercise (with one empty set) to the
// user's running workout.
func (s *Service) AddExercise(ctx context.Context, userID uint32, lang string, in *AddExerciseInput) (*SessionExercise, error) {
	if err := s.checkCatalogExercise(ctx, in.ExerciseID); err != nil {
		return nil, err
	}

	var workoutID uint32
	err := s.db.GetContext(ctx, &workoutID, selectActiveWorkoutIDQuery, userID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var nextPos uint8
	if err := s.db.GetContext(ctx, &nextPos, nextPositionQuery, workoutID); err != nil {
		return nil, err
	}
	if int(nextPos) > MaxExercises {
		return nil, ErrTooManyExercises
	}

	res, err := s.db.ExecContext(ctx, insertAddedExerciseQuery, workoutID, in.ExerciseID, nextPos, in.RestAfterSec)
	if err != nil {
		return nil, err
	}
	wexID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, insertWorkoutSetFactQuery, wexID, 1); err != nil {
		return nil, err
	}
	return s.loadSessionExercise(ctx, userID, uint32(wexID), lang)
}

// Complete finishes the user's running workout.
func (s *Service) Complete(ctx context.Context, userID uint32) error {
	return s.finish(ctx, userID, "completed")
}

// Cancel aborts the user's running workout (the record stays in history
// with the facts collected so far).
func (s *Service) Cancel(ctx context.Context, userID uint32) error {
	return s.finish(ctx, userID, "cancelled")
}

func (s *Service) finish(ctx context.Context, userID uint32, status string) error {
	res, err := s.db.ExecContext(ctx, finishWorkoutQuery, status, userID)
	if err != nil {
		return err
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

// activeWorkoutExercise verifies ownership + in_progress for structural
// changes on a workout exercise.
func (s *Service) activeWorkoutExercise(ctx context.Context, userID uint32, wexID uint32) (uint32, error) {
	var workoutID uint32
	err := s.db.GetContext(ctx, &workoutID, selectWorkoutExerciseActiveQuery, wexID, userID)
	if isNoRows(err) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return workoutID, nil
}

func (s *Service) checkCatalogExercise(ctx context.Context, exerciseID uint16) error {
	var id uint32
	err := s.db.GetContext(ctx, &id, checkCatalogExerciseQuery, exerciseID)
	if isNoRows(err) {
		return ErrInvalidExercise
	}
	return err
}
