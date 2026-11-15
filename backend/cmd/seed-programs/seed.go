package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
)

const exerciseSource = "exercises-dataset"

type seedStats struct {
	Created, Updated int
	Days, Exercises  int
	Sets             int
}

type catalogExercise struct {
	ID         int64
	Tracking   string
	Name       string
}

type seeder struct {
	tx *sql.Tx
}

func (s *seeder) seed(ctx context.Context, progs []manifestProgram, replace bool) (*seedStats, error) {
	catalog, err := s.resolveExercises(ctx, progs)
	if err != nil {
		return nil, err
	}
	if err := validateTracking(progs, catalog); err != nil {
		return nil, err
	}

	if replace {
		res, err := s.tx.ExecContext(ctx, `DELETE FROM programs WHERE owner_id IS NULL`)
		if err != nil {
			return nil, fmt.Errorf("удаление готовых программ: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("replace: удалено готовых программ %d", n)
		}
	}

	st := &seedStats{}
	for pi, p := range progs {
		progID, created, err := s.upsertProgram(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("программа %d (%q): %w", pi+1, p.Name, err)
		}
		if created {
			st.Created++
		} else {
			st.Updated++
		}
		for di, d := range p.Days {
			dayID, err := s.insertDay(ctx, progID, di+1, d.Name)
			if err != nil {
				return nil, fmt.Errorf("программа %q, день %d: %w", p.Name, di+1, err)
			}
			st.Days++
			for ei, e := range d.Exercises {
				peID, err := s.insertExercise(ctx, dayID, ei+1, e, catalog[e.ExternalID])
				if err != nil {
					return nil, fmt.Errorf("программа %q, день %d, упражнение %d: %w", p.Name, di+1, ei+1, err)
				}
				st.Exercises++
				if err := s.insertSets(ctx, peID, e.Sets); err != nil {
					return nil, fmt.Errorf("программа %q, день %d, упражнение %d: %w", p.Name, di+1, ei+1, err)
				}
				st.Sets += len(e.Sets)
			}
		}
	}
	return st, nil
}

func (s *seeder) resolveExercises(ctx context.Context, progs []manifestProgram) (map[string]catalogExercise, error) {
	extIDs := map[string]bool{}
	for _, p := range progs {
		for _, d := range p.Days {
			for _, e := range d.Exercises {
				extIDs[e.ExternalID] = true
			}
		}
	}

	catalog := make(map[string]catalogExercise, len(extIDs))
	for ext := range extIDs {
		var ce catalogExercise
		err := s.tx.QueryRowContext(ctx, `
			SELECT e.id, e.tracking_type, COALESCE(NULLIF(et.name, ''), e.external_id)
			FROM exercises e
			LEFT JOIN exercise_translations et ON et.exercise_id = e.id AND et.lang = 'ru'
			WHERE e.source = ? AND e.external_id = ?`, exerciseSource, ext).
			Scan(&ce.ID, &ce.Tracking, &ce.Name)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("упражнение %s: нет в каталоге (источник %s); выполните make seed-catalog", ext, exerciseSource)
		}
		if err != nil {
			return nil, fmt.Errorf("упражнение %s: %w", ext, err)
		}
		catalog[ext] = ce
	}
	return catalog, nil
}

func validateTracking(progs []manifestProgram, catalog map[string]catalogExercise) error {
	for _, p := range progs {
		for _, d := range p.Days {
			for _, e := range d.Exercises {
				ce := catalog[e.ExternalID]
				for si, set := range e.Sets {
					if err := checkSetByTracking(p.Name, d.Name, e.ExternalID, ce, si+1, set); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func checkSetByTracking(progName, dayName, extID string, ce catalogExercise, setNum int, set manifestSet) error {
	where := fmt.Sprintf("%q → %q → %s, подход %d", progName, dayName, extID, setNum)
	track := ce.Tracking

	switch track {
	case "weight_reps":
		if set.Reps == nil {
			return fmt.Errorf("%s (%s): задайте reps", where, track)
		}
		if set.DurationSec != nil || set.DistanceM != nil {
			return fmt.Errorf("%s (%s): duration_sec/distance_m не задаются", where, track)
		}
	case "reps":
		if set.Reps == nil {
			return fmt.Errorf("%s (%s): задайте reps", where, track)
		}
		if set.WeightKg != nil || set.DurationSec != nil || set.DistanceM != nil {
			return fmt.Errorf("%s (%s): упражнение с собственным весом — weight_kg/duration_sec/distance_m не задаются", where, track)
		}
	case "time":
		if set.DurationSec == nil {
			return fmt.Errorf("%s (%s): задайте duration_sec", where, track)
		}
		if set.Reps != nil || set.WeightKg != nil || set.DistanceM != nil {
			return fmt.Errorf("%s (%s): reps/weight_kg/distance_m не задаются", where, track)
		}
	case "distance_time":
		if set.DistanceM == nil {
			return fmt.Errorf("%s (%s): задайте distance_m", where, track)
		}
		if set.Reps != nil || set.WeightKg != nil {
			return fmt.Errorf("%s (%s): reps/weight_kg не задаются", where, track)
		}
	default:
		return fmt.Errorf("%s: неизвестный tracking_type %q упражнения %q", where, track, ce.Name)
	}
	return nil
}

func (s *seeder) upsertProgram(ctx context.Context, p manifestProgram) (int64, bool, error) {
	var existing int64
	err := s.tx.QueryRowContext(ctx,
		`SELECT id FROM programs WHERE owner_id IS NULL AND name = ? FOR UPDATE`, p.Name).
		Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := s.tx.ExecContext(ctx, `
			INSERT INTO programs (owner_id, name, description, author_name, level, is_published)
			VALUES (NULL, ?, ?, ?, ?, TRUE)`,
			p.Name, nullStr(p.Description), nullStr(p.AuthorName), p.Level)
		if err != nil {
			return 0, false, fmt.Errorf("insert: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	case err != nil:
		return 0, false, err
	}

	if _, err := s.tx.ExecContext(ctx, `
		UPDATE programs
		SET name = ?, description = ?, author_name = ?, level = ?, is_published = TRUE
		WHERE id = ? AND owner_id IS NULL`,
		p.Name, nullStr(p.Description), nullStr(p.AuthorName), p.Level, existing); err != nil {
		return 0, false, fmt.Errorf("update: %w", err)
	}
	if _, err := s.tx.ExecContext(ctx, `DELETE FROM program_days WHERE program_id = ?`, existing); err != nil {
		return 0, false, fmt.Errorf("delete days: %w", err)
	}
	return existing, false, nil
}

func (s *seeder) insertDay(ctx context.Context, progID int64, position int, name string) (int64, error) {
	res, err := s.tx.ExecContext(ctx,
		`INSERT INTO program_days (program_id, position, name) VALUES (?, ?, ?)`,
		progID, position, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *seeder) insertExercise(ctx context.Context, dayID int64, position int, e manifestExercise, ce catalogExercise) (int64, error) {
	var restAfter any
	if e.RestAfterSec != nil {
		restAfter = *e.RestAfterSec
	}
	res, err := s.tx.ExecContext(ctx, `
		INSERT INTO program_exercises (program_day_id, exercise_id, position, rest_after_sec)
		VALUES (?, ?, ?, ?)`,
		dayID, ce.ID, position, restAfter)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *seeder) insertSets(ctx context.Context, peID int64, sets []manifestSet) error {
	stmt, err := s.tx.PrepareContext(ctx, `
		INSERT INTO program_sets (program_exercise_id, set_number, reps, weight_kg, duration_sec, distance_m, rest_sec)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, set := range sets {
		var reps, duration, distance, rest any
		var weight any
		if set.Reps != nil {
			reps = *set.Reps
		}
		if set.WeightKg != nil {
			weight = *set.WeightKg
		}
		if set.DurationSec != nil {
			duration = *set.DurationSec
		}
		if set.DistanceM != nil {
			distance = *set.DistanceM
		}
		if set.RestSec != nil {
			rest = *set.RestSec
		}
		if _, err := stmt.ExecContext(ctx, peID, i+1, reps, weight, duration, distance, rest); err != nil {
			return err
		}
	}
	return nil
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
