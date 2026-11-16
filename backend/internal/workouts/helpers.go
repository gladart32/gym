package workouts

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

const mediaURLPrefix = "/media/"

// mediaURL turns a stored relative path into a servable URL
// (same rules as the exercises domain).
func mediaURL(path *string) string {
	if path == nil || *path == "" {
		return ""
	}
	p := *path
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "/") {
		return p
	}
	return mediaURLPrefix + p
}

// exerciseListQuery renders the session exercise list query with the
// localized equipment column (same rule as programs.nameColumn).
func exerciseListQuery(lang string) string {
	column := "name_ru"
	if lang == "en" {
		column = "name_en"
	}
	return fmt.Sprintf(selectSessionExercisesQueryTmpl, column)
}

// selectExerciseOwnerQuery resolves the workout behind a workout exercise
// and enforces ownership.
const selectExerciseOwnerQuery = `
	SELECT wex.workout_id
	FROM workout_exercises wex
	JOIN workouts w ON w.id = wex.workout_id
	WHERE wex.id = ? AND w.user_id = ?`

// selectSetOwnerQuery resolves the workout behind a set and enforces ownership.
const selectSetOwnerQuery = `
	SELECT ws.workout_exercise_id AS workout_exercise_id
	FROM workout_sets ws
	JOIN workout_exercises wex ON wex.id = ws.workout_exercise_id
	JOIN workouts w ON w.id = wex.workout_id
	WHERE ws.id = ? AND w.user_id = ?`
