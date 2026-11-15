package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

const (
	maxDays          = 50
	maxExercises     = 50
	maxSets          = 50
	maxNameLen       = 255
	maxDescriptionSz = 65535
	maxReps          = 65535
	maxWeightKg      = 500
	minWeightKg      = -300
	maxRestSec       = 65535
)

type manifestSet struct {
	Reps        *uint    `json:"reps"`
	WeightKg    *float64 `json:"weight_kg"`
	DurationSec *uint    `json:"duration_sec"`
	DistanceM   *uint    `json:"distance_m"`
	RestSec     *uint    `json:"rest_sec"`
}

type manifestExercise struct {
	ExternalID   string        `json:"external_id"`
	RestAfterSec *uint         `json:"rest_after_sec"`
	Sets         []manifestSet `json:"sets"`
}

type manifestDay struct {
	Name      string             `json:"name"`
	Exercises []manifestExercise `json:"exercises"`
}

type manifestProgram struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	AuthorName  string        `json:"author_name"`
	Level       string        `json:"level"`
	Days        []manifestDay `json:"days"`
}

var validLevels = map[string]bool{
	"beginner":     true,
	"intermediate": true,
	"advanced":     true,
}

func loadManifest(path string) ([]manifestProgram, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file struct {
		Comment  string            `json:"$comment"`
		Programs []manifestProgram `json:"programs"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	return file.Programs, nil
}

func validateManifest(progs []manifestProgram) error {
	if len(progs) == 0 {
		return fmt.Errorf("манифест пуст")
	}
	seen := map[string]bool{}
	for i, p := range progs {
		where := fmt.Sprintf("программа %d (%q)", i+1, p.Name)
		if p.Name == "" || len(p.Name) > maxNameLen {
			return fmt.Errorf("%s: имя должно быть 1..%d символов", where, maxNameLen)
		}
		if seen[p.Name] {
			return fmt.Errorf("%s: имя программы повторяется", where)
		}
		seen[p.Name] = true
		if !validLevels[p.Level] {
			return fmt.Errorf("%s: недопустимый уровень %q (beginner/intermediate/advanced)", where, p.Level)
		}
		if p.AuthorName != "" && len(p.AuthorName) > maxNameLen {
			return fmt.Errorf("%s: автор длиннее %d символов", where, maxNameLen)
		}
		if len(p.Description) > maxDescriptionSz {
			return fmt.Errorf("%s: описание длиннее %d байт", where, maxDescriptionSz)
		}
		if len(p.Days) == 0 || len(p.Days) > maxDays {
			return fmt.Errorf("%s: дней должно быть 1..%d", where, maxDays)
		}
		for j, d := range p.Days {
			dw := fmt.Sprintf("%s, день %d (%q)", where, j+1, d.Name)
			if d.Name == "" || len(d.Name) > maxNameLen {
				return fmt.Errorf("%s: имя дня 1..%d символов", dw, maxNameLen)
			}
			if len(d.Exercises) == 0 || len(d.Exercises) > maxExercises {
				return fmt.Errorf("%s: упражнений в дне 1..%d", dw, maxExercises)
			}
			for k, e := range d.Exercises {
				ew := fmt.Sprintf("%s, упражнение %d", dw, k+1)
				if e.ExternalID == "" {
					return fmt.Errorf("%s: пустой external_id", ew)
				}
				if len(e.Sets) == 0 || len(e.Sets) > maxSets {
					return fmt.Errorf("%s: подходов должно быть 1..%d", ew, maxSets)
				}
				if err := checkSetRanges(ew, e); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkSetRanges(where string, e manifestExercise) error {
	if e.RestAfterSec != nil && *e.RestAfterSec > maxRestSec {
		return fmt.Errorf("%s: rest_after_sec > %d", where, maxRestSec)
	}
	for si, set := range e.Sets {
		sw := fmt.Sprintf("%s, подход %d", where, si+1)
		if set.Reps != nil && (*set.Reps == 0 || *set.Reps > maxReps) {
			return fmt.Errorf("%s: reps должно быть 1..%d", sw, maxReps)
		}
		if set.DurationSec != nil && *set.DurationSec == 0 {
			return fmt.Errorf("%s: duration_sec должно быть > 0", sw)
		}
		if set.DistanceM != nil && *set.DistanceM == 0 {
			return fmt.Errorf("%s: distance_m должно быть > 0", sw)
		}
		if set.WeightKg != nil && (*set.WeightKg < minWeightKg || *set.WeightKg > maxWeightKg) {
			return fmt.Errorf("%s: weight_kg должно быть в диапазоне %d..%d", sw, minWeightKg, maxWeightKg)
		}
		if set.RestSec != nil && *set.RestSec > maxRestSec {
			return fmt.Errorf("%s: rest_sec > %d", sw, maxRestSec)
		}
	}
	return nil
}

func countDays(progs []manifestProgram) int {
	n := 0
	for _, p := range progs {
		n += len(p.Days)
	}
	return n
}

func countExercises(progs []manifestProgram) int {
	n := 0
	for _, p := range progs {
		for _, d := range p.Days {
			n += len(d.Exercises)
		}
	}
	return n
}

func countSets(progs []manifestProgram) int {
	n := 0
	for _, p := range progs {
		for _, d := range p.Days {
			for _, e := range d.Exercises {
				n += len(e.Sets)
			}
		}
	}
	return n
}
