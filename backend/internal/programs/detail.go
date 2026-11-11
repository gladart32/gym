package programs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// ErrNotFound means the program does not exist or is not visible
// to the requesting user (hidden ready program).
var ErrNotFound = errors.New("program not found")

// ErrForbidden means the program exists but the user may not modify it.
var ErrForbidden = errors.New("forbidden: only the owner or admin can modify this program")

// Set is one planned set of a program exercise.
type Set struct {
	ID          uint32   `json:"id"`
	SetNumber   uint8    `json:"set_number"`
	Reps        *uint16  `json:"reps"`
	WeightKg    *float64 `json:"weight_kg"`
	DurationSec *uint32  `json:"duration_sec"`
	DistanceM   *uint32  `json:"distance_m"`
	RestSec     *uint16  `json:"rest_sec"`
}

// Exercise is one catalog exercise placed inside a program day.
type Exercise struct {
	ID           uint32  `json:"id"` // program_exercises.id
	ExerciseID   uint16  `json:"exercise_id"`
	Name         string  `json:"name"`
	Equipment    string  `json:"equipment"`
	TrackingType string  `json:"tracking_type"`
	Position     uint8   `json:"position"`
	RestAfterSec *uint16 `json:"rest_after_sec"`
	ThumbnailURL string  `json:"thumbnail_url"`
	GifURL       string  `json:"gif_url"`
	Sets         []Set   `json:"sets"`
}

// Day is one training day (group of exercises) of the program.
type Day struct {
	ID        uint32     `json:"id"`
	Position  uint8      `json:"position"`
	Name      string     `json:"name"`
	Exercises []Exercise `json:"exercises"`
}

// Detail is the full program tree with permission flags.
type Detail struct {
	ID           uint32    `json:"id"`
	OwnerID      *uint32   `json:"owner_id"`
	CopiedFromID *uint32   `json:"copied_from_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	AuthorName   *string   `json:"author_name"`
	Level        *string   `json:"level"`
	IsPublished  bool      `json:"is_published"`
	IsMine       bool      `json:"is_mine"`
	IsReady      bool      `json:"is_ready"`
	CanEdit      bool      `json:"can_edit"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Days         []Day     `json:"days"`
}

const detailQuery = `
	SELECT id, owner_id, copied_from_id, name, description, author_name,
	       level, is_published, created_at, updated_at
	FROM programs
	WHERE id = ?`

const detailDaysQuery = `
	SELECT id, position, name
	FROM program_days
	WHERE program_id = ?
	ORDER BY position, id`

const detailExercisesQueryTmpl = `
	SELECT
		pe.id,
		pe.exercise_id,
		pe.program_day_id,
		pe.position,
		pe.rest_after_sec,
		t.name,
		e.tracking_type,
		eq.%[1]s AS equipment_name,
		(SELECT g.path FROM exercise_media g
		 WHERE g.exercise_id = e.id AND g.type = 'gif'
		 ORDER BY g.sort_order, g.id LIMIT 1) AS gif_path,
		(SELECT th.path FROM exercise_media th
		 WHERE th.exercise_id = e.id AND th.type = 'thumbnail'
		 ORDER BY th.sort_order, th.id LIMIT 1) AS thumbnail_path
	FROM program_exercises pe
	JOIN exercises e ON e.id = pe.exercise_id
	JOIN exercise_translations t ON t.exercise_id = e.id AND t.lang = ?
	JOIN equipment eq ON eq.id = e.equipment_id
	WHERE pe.program_day_id IN (?)
	ORDER BY pe.position, pe.id`

const detailSetsQuery = `
	SELECT id, program_exercise_id, set_number, reps, weight_kg,
	       duration_sec, distance_m, rest_sec
	FROM program_sets
	WHERE program_exercise_id IN (?)
	ORDER BY set_number`

type detailRow struct {
	ID           uint32    `db:"id"`
	OwnerID      *uint32   `db:"owner_id"`
	CopiedFromID *uint32   `db:"copied_from_id"`
	Name         string    `db:"name"`
	Description  *string   `db:"description"`
	AuthorName   *string   `db:"author_name"`
	Level        *string   `db:"level"`
	IsPublished  bool      `db:"is_published"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type dayRow struct {
	ID       uint32 `db:"id"`
	Position uint8  `db:"position"`
	Name     string `db:"name"`
}

type exerciseRow struct {
	ID            uint32  `db:"id"`
	ExerciseID    uint16  `db:"exercise_id"`
	ProgramDayID  uint32  `db:"program_day_id"`
	Position      uint8   `db:"position"`
	RestAfterSec  *uint16 `db:"rest_after_sec"`
	Name          string  `db:"name"`
	TrackingType  string  `db:"tracking_type"`
	EquipmentName string  `db:"equipment_name"`
	GifPath       *string `db:"gif_path"`
	ThumbnailPath *string `db:"thumbnail_path"`
}

type setRow struct {
	ID                uint32   `db:"id"`
	ProgramExerciseID uint32   `db:"program_exercise_id"`
	SetNumber         uint8    `db:"set_number"`
	Reps              *uint16  `db:"reps"`
	WeightKg          *float64 `db:"weight_kg"`
	DurationSec       *uint32  `db:"duration_sec"`
	DistanceM         *uint32  `db:"distance_m"`
	RestSec           *uint16  `db:"rest_sec"`
}

// detailAccess reports whether the user may see the program row.
// Hidden ready programs are indistinguishable from missing ones.
func (r *detailRow) visibleTo(userID uint32, isAdmin bool) bool {
	if r.OwnerID != nil {
		return *r.OwnerID == userID
	}
	return r.IsPublished || isAdmin
}

func (r *detailRow) canEdit(userID uint32, isAdmin bool) bool {
	if r.OwnerID != nil {
		return *r.OwnerID == userID
	}
	return isAdmin
}

// Detail returns the full program tree. Access: own personal programs,
// published ready programs for everyone, ready drafts for admins only.
func (s *Service) Detail(ctx context.Context, userID uint32, isAdmin bool, id uint32, lang string) (*Detail, error) {
	var row detailRow
	err := s.db.GetContext(ctx, &row, detailQuery, id)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !row.visibleTo(userID, isAdmin) {
		return nil, ErrNotFound
	}

	d := &Detail{
		ID:           row.ID,
		OwnerID:      row.OwnerID,
		CopiedFromID: row.CopiedFromID,
		Name:         row.Name,
		Description:  row.Description,
		AuthorName:   row.AuthorName,
		Level:        row.Level,
		IsPublished:  row.IsPublished,
		IsMine:       row.OwnerID != nil && *row.OwnerID == userID,
		IsReady:      row.OwnerID == nil,
		CanEdit:      row.canEdit(userID, isAdmin),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}

	var days []dayRow
	if err := s.db.SelectContext(ctx, &days, detailDaysQuery, id); err != nil {
		return nil, err
	}

	dayIDs := make([]uint32, 0, len(days))
	dayIndex := make(map[uint32]int, len(days))
	for i, dr := range days {
		dayIDs = append(dayIDs, dr.ID)
		dayIndex[dr.ID] = i
	}

	d.Days = make([]Day, 0, len(days))
	for _, dr := range days {
		d.Days = append(d.Days, Day{
			ID:        dr.ID,
			Position:  dr.Position,
			Name:      dr.Name,
			Exercises: []Exercise{},
		})
	}

	if len(dayIDs) == 0 {
		return d, nil
	}

	var exRows []exerciseRow
	query, args, err := sqlx.In(fmt.Sprintf(detailExercisesQueryTmpl, nameColumn(lang)), lang, dayIDs)
	if err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &exRows, query, args...); err != nil {
		return nil, err
	}

	// exIndex points at the per-day slice position of each exercise.
	type exLocation struct {
		dayIdx int
		exIdx  int
	}
	exIndex := make(map[uint32]exLocation, len(exRows))
	exIDs := make([]uint32, 0, len(exRows))
	for _, er := range exRows {
		dayIdx := dayIndex[er.ProgramDayID]
		exIdx := len(d.Days[dayIdx].Exercises)
		d.Days[dayIdx].Exercises = append(d.Days[dayIdx].Exercises, Exercise{
			ID:           er.ID,
			ExerciseID:   er.ExerciseID,
			Name:         er.Name,
			Equipment:    er.EquipmentName,
			TrackingType: er.TrackingType,
			Position:     er.Position,
			RestAfterSec: er.RestAfterSec,
			ThumbnailURL: mediaURL(er.ThumbnailPath),
			GifURL:       mediaURL(er.GifPath),
			Sets:         []Set{},
		})
		exIndex[er.ID] = exLocation{dayIdx: dayIdx, exIdx: exIdx}
		exIDs = append(exIDs, er.ID)
	}

	if len(exIDs) == 0 {
		return d, nil
	}

	var setRows []setRow
	setQuery, setArgs, err := sqlx.In(detailSetsQuery, exIDs)
	if err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &setRows, setQuery, setArgs...); err != nil {
		return nil, err
	}
	for _, sr := range setRows {
		loc, ok := exIndex[sr.ProgramExerciseID]
		if !ok {
			continue
		}
		d.Days[loc.dayIdx].Exercises[loc.exIdx].Sets = append(d.Days[loc.dayIdx].Exercises[loc.exIdx].Sets, Set{
			ID:          sr.ID,
			SetNumber:   sr.SetNumber,
			Reps:        sr.Reps,
			WeightKg:    sr.WeightKg,
			DurationSec: sr.DurationSec,
			DistanceM:   sr.DistanceM,
			RestSec:     sr.RestSec,
		})
	}

	return d, nil
}
