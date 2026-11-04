package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rawExercise — запись из exercises-dataset/data/exercises.json.
type rawExercise struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Category         string              `json:"category"`
	BodyPart         string              `json:"body_part"`
	Equipment        string              `json:"equipment"`
	Target           string              `json:"target"`
	MuscleGroup      string              `json:"muscle_group"`
	SecondaryMuscles []string            `json:"secondary_muscles"`
	Instructions     map[string]string   `json:"instructions"`
	InstructionSteps map[string][]string `json:"instruction_steps"`
	Image            string              `json:"image"`
	GifURL           string              `json:"gif_url"`
	MediaID          string              `json:"media_id"`
	Attribution      string              `json:"attribution"`
}

// ruExercise — запись из translations/ru/exercises.json.
type ruExercise struct {
	Name         string `json:"name"`
	Instructions string `json:"instructions,omitempty"` // только если исправлено
}

type muscleTr struct {
	RU        string `json:"ru"`
	Canonical string `json:"canonical"`
}

type sourceData struct {
	Exercises  []rawExercise
	RU         map[string]ruExercise
	BodyParts  map[string]string // code -> ru
	Equipment  map[string]string // code -> ru
	Muscles    map[string]muscleTr
	DatasetDir string
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func loadSources(datasetDir, trDir string) (*sourceData, error) {
	s := &sourceData{DatasetDir: datasetDir}

	if err := readJSON(filepath.Join(datasetDir, "data", "exercises.json"), &s.Exercises); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(trDir, "exercises.json"), &s.RU); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(trDir, "body_parts.json"), &s.BodyParts); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(trDir, "equipment.json"), &s.Equipment); err != nil {
		return nil, err
	}

	// muscles.json содержит служебный ключ "_comment" со строкой — читаем через RawMessage.
	var rawMuscles map[string]json.RawMessage
	if err := readJSON(filepath.Join(trDir, "muscles.json"), &rawMuscles); err != nil {
		return nil, err
	}
	s.Muscles = make(map[string]muscleTr, len(rawMuscles))
	for k, v := range rawMuscles {
		if strings.HasPrefix(k, "_") {
			continue
		}
		var m muscleTr
		if err := json.Unmarshal(v, &m); err != nil {
			return nil, fmt.Errorf("muscles.json %q: %w", k, err)
		}
		s.Muscles[k] = m
	}
	return s, nil
}

// validate проверяет, что для всех значений из датасета есть переводы,
// а медиафайлы существуют (если checkMedia).
func (s *sourceData) validate(checkMedia bool) error {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }

	for code, m := range s.Muscles {
		if _, ok := s.Muscles[m.Canonical]; !ok {
			add("muscles.json: canonical %q (у %q) не описан отдельным ключом", m.Canonical, code)
		}
	}

	seen := make(map[string]bool, len(s.Exercises))
	for _, e := range s.Exercises {
		if seen[e.ID] {
			add("exercise %s: повторяющийся id", e.ID)
		}
		seen[e.ID] = true

		if _, ok := s.BodyParts[e.Category]; !ok {
			add("exercise %s: нет перевода части тела %q", e.ID, e.Category)
		}
		if _, ok := s.Equipment[e.Equipment]; !ok {
			add("exercise %s: нет перевода оборудования %q", e.ID, e.Equipment)
		}
		for _, m := range append([]string{e.Target, e.MuscleGroup}, e.SecondaryMuscles...) {
			if m == "" {
				continue
			}
			if _, ok := s.Muscles[m]; !ok {
				add("exercise %s: нет перевода мышцы %q", e.ID, m)
			}
		}
		if r, ok := s.RU[e.ID]; !ok || r.Name == "" {
			add("exercise %s: нет русского названия", e.ID)
		}
		if e.Instructions["en"] == "" {
			add("exercise %s: нет инструкции en", e.ID)
		}
		if checkMedia {
			for _, p := range []string{e.GifURL, e.Image} {
				if p == "" {
					add("exercise %s: пустой путь к медиа", e.ID)
					continue
				}
				if _, err := os.Stat(filepath.Join(s.DatasetDir, p)); err != nil {
					add("exercise %s: нет файла %s", e.ID, p)
				}
			}
		}
	}

	if len(errs) > 0 {
		const max = 20
		if len(errs) > max {
			errs = append(errs[:max], fmt.Sprintf("... и ещё %d", len(errs)-max))
		}
		return fmt.Errorf("ошибки валидации:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// fixName исправляет битую кодировку в исходных названиях ("45в°" -> "45°").
func fixName(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "в°", "°"))
}

func titleFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// canonicalMuscles возвращает канонический код целевой мышцы и
// отсортированный по порядку появления список вспомогательных без дублей и без целевой.
func (s *sourceData) canonicalMuscles(e rawExercise) (target string, secondary []string) {
	target = s.Muscles[e.Target].Canonical
	seen := map[string]bool{target: true}
	for _, m := range append([]string{e.MuscleGroup}, e.SecondaryMuscles...) {
		if m == "" {
			continue
		}
		c := s.Muscles[m].Canonical
		if !seen[c] {
			seen[c] = true
			secondary = append(secondary, c)
		}
	}
	return target, secondary
}
