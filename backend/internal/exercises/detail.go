package exercises

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotFound means the exercise does not exist or is hidden (is_active = 0).
var ErrNotFound = errors.New("exercise not found")

// MediaItem is a single media file of the detail card.
type MediaItem struct {
	Type        string  `json:"type"`
	URL         string  `json:"url"`
	Width       *uint16 `json:"width"`
	Height      *uint16 `json:"height"`
	Attribution *string `json:"attribution"`
}

// Detail is the full exercise card.
type Detail struct {
	ID               uint16      `json:"id"`
	Name             string      `json:"name"`
	Instructions     *string     `json:"instructions"`
	Steps            []string    `json:"steps"`
	BodyPart         Ref         `json:"body_part"`
	Equipment        Ref         `json:"equipment"`
	TargetMuscle     Ref         `json:"target_muscle"`
	SecondaryMuscles []Ref       `json:"secondary_muscles"`
	TrackingType     string      `json:"tracking_type"`
	IsFavorite       bool        `json:"is_favorite"`
	Media            []MediaItem `json:"media"`
}

const detailQueryTmpl = `
	SELECT
		e.id,
		t.name,
		t.instructions,
		t.steps,
		e.body_part_id,
		bp.%[1]s AS body_part_name,
		e.equipment_id,
		eq.%[1]s AS equipment_name,
		e.target_muscle_id AS muscle_id,
		m.%[1]s AS muscle_name,
		e.tracking_type,
		fav.user_id IS NOT NULL AS is_favorite
	FROM exercises e
	JOIN exercise_translations t ON t.exercise_id = e.id AND t.lang = ?
	LEFT JOIN exercise_favorites fav ON fav.exercise_id = e.id AND fav.user_id = ?
	JOIN body_parts bp ON bp.id = e.body_part_id
	JOIN equipment eq ON eq.id = e.equipment_id
	JOIN muscles m ON m.id = e.target_muscle_id
	WHERE e.id = ? AND e.is_active = 1`

const detailMediaQuery = `
	SELECT type, path, width, height, attribution
	FROM exercise_media
	WHERE exercise_id = ?
	ORDER BY type, sort_order, id`

const detailMusclesQueryTmpl = `
	SELECT m.id, m.%[1]s AS name
	FROM exercise_secondary_muscles sm
	JOIN muscles m ON m.id = sm.muscle_id
	WHERE sm.exercise_id = ?
	ORDER BY m.id`

type detailRow struct {
	ID            uint16  `db:"id"`
	Name          string  `db:"name"`
	Instructions  *string `db:"instructions"`
	Steps         []byte  `db:"steps"`
	BodyPartID    uint16  `db:"body_part_id"`
	BodyPartName  string  `db:"body_part_name"`
	EquipmentID   uint16  `db:"equipment_id"`
	EquipmentName string  `db:"equipment_name"`
	MuscleID      uint16  `db:"muscle_id"`
	MuscleName    string  `db:"muscle_name"`
	TrackingType  string  `db:"tracking_type"`
	IsFavorite    bool    `db:"is_favorite"`
}

type mediaRow struct {
	Type        string  `db:"type"`
	Path        string  `db:"path"`
	Width       *uint16 `db:"width"`
	Height      *uint16 `db:"height"`
	Attribution *string `db:"attribution"`
}

// Detail returns the full card of an active exercise with the favorite
// mark of the given user.
func (s *Service) Detail(ctx context.Context, userID, id uint32, lang string) (*Detail, error) {
	col := nameColumn(lang)

	var row detailRow
	err := s.db.GetContext(ctx, &row, fmt.Sprintf(detailQueryTmpl, col), lang, userID, id)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	d := &Detail{
		ID:           row.ID,
		Name:         row.Name,
		Instructions: row.Instructions,
		BodyPart:     Ref{ID: row.BodyPartID, Name: row.BodyPartName},
		Equipment:    Ref{ID: row.EquipmentID, Name: row.EquipmentName},
		TargetMuscle: Ref{ID: row.MuscleID, Name: row.MuscleName},
		TrackingType: row.TrackingType,
		IsFavorite:   row.IsFavorite,
	}
	if row.Steps != nil {
		var steps []string
		if err := json.Unmarshal(row.Steps, &steps); err != nil {
			return nil, err
		}
		d.Steps = steps
	}

	var muscles []Ref
	if err := s.db.SelectContext(ctx, &muscles, fmt.Sprintf(detailMusclesQueryTmpl, col), id); err != nil {
		return nil, err
	}
	d.SecondaryMuscles = muscles
	if d.SecondaryMuscles == nil {
		d.SecondaryMuscles = []Ref{}
	}

	var media []mediaRow
	if err := s.db.SelectContext(ctx, &media, detailMediaQuery, id); err != nil {
		return nil, err
	}
	d.Media = make([]MediaItem, 0, len(media))
	for _, m := range media {
		d.Media = append(d.Media, MediaItem{
			Type:        m.Type,
			URL:         mediaURL(&m.Path),
			Width:       m.Width,
			Height:      m.Height,
			Attribution: m.Attribution,
		})
	}

	return d, nil
}
