package exercises

import (
	"context"
	"fmt"
	"strings"
)

// Ref is a localized catalog dictionary entry (body part, equipment, muscle).
type Ref struct {
	ID   uint16 `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// Media holds the visual links of a list item.
type Media struct {
	GifURL       string  `json:"gif_url"`
	ThumbnailURL string  `json:"thumbnail_url"`
	Attribution  *string `json:"attribution"`
}

// Item is one exercise card in the list response.
type Item struct {
	ID           uint16 `json:"id"`
	Name         string `json:"name"`
	BodyPart     Ref    `json:"body_part"`
	Equipment    Ref    `json:"equipment"`
	TargetMuscle Ref    `json:"target_muscle"`
	TrackingType string `json:"tracking_type"`
	IsFavorite   bool   `json:"is_favorite"`
	Media        Media  `json:"media"`
}

// ListResult is the paginated list response.
type ListResult struct {
	Items   []Item `json:"items"`
	Total   int64  `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Pages   int    `json:"pages"`
}

// ListFilter carries all parsed query parameters of the list endpoint.
type ListFilter struct {
	Q             string
	BodyPartIDs   []uint16
	EquipmentIDs  []uint16
	MuscleIDs     []uint16
	TrackingTypes []string
	MyEquipment   bool
	FavoritesOnly bool
	Sort          string // name | newest
	Order         string // asc | desc
	Page          int
	PerPage       int
	UserID        uint32 // for MyEquipment / FavoritesOnly / is_favorite
	Lang          string
}

const (
	DefaultPerPage = 20
	MaxPerPage     = 50

	// ngram_token_size of the FULLTEXT index: queries shorter than two
	// runes cannot match it and fall back to LIKE.
	ngramTokenSize = 2
)

const listSelectTmpl = `
	SELECT
		e.id,
		t.name,
		e.body_part_id,
		bp.%[1]s AS body_part_name,
		e.equipment_id,
		eq.%[1]s AS equipment_name,
		e.target_muscle_id AS muscle_id,
		m.%[1]s AS muscle_name,
		e.tracking_type,
		fav.user_id IS NOT NULL AS is_favorite,
		(SELECT g.path FROM exercise_media g
		 WHERE g.exercise_id = e.id AND g.type = 'gif'
		 ORDER BY g.sort_order, g.id LIMIT 1) AS gif_path,
		(SELECT g.attribution FROM exercise_media g
		 WHERE g.exercise_id = e.id AND g.type = 'gif'
		 ORDER BY g.sort_order, g.id LIMIT 1) AS attribution,
		(SELECT th.path FROM exercise_media th
		 WHERE th.exercise_id = e.id AND th.type = 'thumbnail'
		 ORDER BY th.sort_order, th.id LIMIT 1) AS thumbnail_path
	FROM exercises e
	JOIN exercise_translations t ON t.exercise_id = e.id AND t.lang = ?
	LEFT JOIN exercise_favorites fav ON fav.exercise_id = e.id AND fav.user_id = ?
	JOIN body_parts bp ON bp.id = e.body_part_id
	JOIN equipment eq ON eq.id = e.equipment_id
	JOIN muscles m ON m.id = e.target_muscle_id
	WHERE e.is_active = 1`

const listCountQuery = `
	SELECT COUNT(*)
	FROM exercises e
	JOIN exercise_translations t ON t.exercise_id = e.id AND t.lang = ?
	LEFT JOIN exercise_favorites fav ON fav.exercise_id = e.id AND fav.user_id = ?
	WHERE e.is_active = 1`

type listItemRow struct {
	ID            uint16  `db:"id"`
	Name          string  `db:"name"`
	BodyPartID    uint16  `db:"body_part_id"`
	BodyPartName  string  `db:"body_part_name"`
	EquipmentID   uint16  `db:"equipment_id"`
	EquipmentName string  `db:"equipment_name"`
	MuscleID      uint16  `db:"muscle_id"`
	MuscleName    string  `db:"muscle_name"`
	TrackingType  string  `db:"tracking_type"`
	IsFavorite    bool    `db:"is_favorite"`
	GifPath       *string `db:"gif_path"`
	ThumbnailPath *string `db:"thumbnail_path"`
	Attribution   *string `db:"attribution"`
}

// List returns a page of active exercises matching the filter.
func (s *Service) List(ctx context.Context, f *ListFilter) (*ListResult, error) {
	f.Page = max(f.Page, 1)
	f.PerPage = min(max(f.PerPage, 1), MaxPerPage)

	cond, condArgs := f.conditions()

	countArgs := append([]any{f.Lang, f.UserID}, condArgs...)
	var total int64
	if err := s.db.GetContext(ctx, &total, listCountQuery+cond, countArgs...); err != nil {
		return nil, err
	}

	offset := (f.Page - 1) * f.PerPage
	args := append([]any{f.Lang, f.UserID}, condArgs...)
	args = append(args, f.PerPage, offset)

	col := nameColumn(f.Lang)
	query := fmt.Sprintf(listSelectTmpl, col, col, col) + cond + f.orderClause() + " LIMIT ? OFFSET ?"

	var rows []listItemRow
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(rows))
	for _, r := range rows {
		items = append(items, Item{
			ID:           r.ID,
			Name:         r.Name,
			BodyPart:     Ref{ID: r.BodyPartID, Name: r.BodyPartName},
			Equipment:    Ref{ID: r.EquipmentID, Name: r.EquipmentName},
			TargetMuscle: Ref{ID: r.MuscleID, Name: r.MuscleName},
			TrackingType: r.TrackingType,
			IsFavorite:   r.IsFavorite,
			Media: Media{
				GifURL:       mediaURL(r.GifPath),
				ThumbnailURL: mediaURL(r.ThumbnailPath),
				Attribution:  r.Attribution,
			},
		})
	}

	pages := 0
	if total > 0 {
		pages = int((total-1)/int64(f.PerPage)) + 1
	}

	return &ListResult{
		Items:   items,
		Total:   total,
		Page:    f.Page,
		PerPage: f.PerPage,
		Pages:   pages,
	}, nil
}

// conditions builds the dynamic WHERE suffix (each entry starts with " AND")
// with its placeholder arguments.
func (f *ListFilter) conditions() (string, []any) {
	var b strings.Builder
	var args []any

	addIn := func(column string, values []any) {
		if len(values) == 0 {
			return
		}
		ph := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
		b.WriteString(" AND " + column + " IN (" + ph + ")")
		args = append(args, values...)
	}

	addIn("e.body_part_id", uint16s(f.BodyPartIDs))
	addIn("e.equipment_id", uint16s(f.EquipmentIDs))
	addIn("e.target_muscle_id", uint16s(f.MuscleIDs))
	addIn("e.tracking_type", stringsToAny(f.TrackingTypes))

	if f.MyEquipment {
		b.WriteString(` AND (
			e.equipment_id IN (SELECT equipment_id FROM user_equipment WHERE user_id = ?)
			OR e.equipment_id = (SELECT id FROM equipment WHERE code = 'body weight'))`)
		args = append(args, f.UserID)
	}

	if f.FavoritesOnly {
		b.WriteString(" AND fav.user_id IS NOT NULL")
	}

	if cond, arg, ok := f.searchCondition(); ok {
		b.WriteString(cond)
		args = append(args, arg)
	}

	return b.String(), args
}

// searchCondition returns a name condition: FULLTEXT boolean mode for queries
// the ngram index can match, LIKE for the rest (short or operator-only input).
func (f *ListFilter) searchCondition() (string, any, bool) {
	q := strings.TrimSpace(f.Q)
	if q == "" {
		return "", nil, false
	}
	if ftq := booleanModeQuery(q); ftq != "" && len([]rune(q)) >= ngramTokenSize {
		return " AND MATCH(t.name) AGAINST(? IN BOOLEAN MODE)", ftq, true
	}
	return " AND t.name LIKE ?", "%" + escapeLike(q) + "%", true
}

// booleanModeQuery strips FULLTEXT boolean operators and rebuilds the query
// as a plain AND of tokens; empty when nothing usable remains.
func booleanModeQuery(q string) string {
	const operators = "+-<>()~*\"@\""
	tokens := strings.Fields(q)
	out := make([]string, 0, len(tokens))
	for _, tk := range tokens {
		tk = strings.Map(func(r rune) rune {
			if strings.ContainsRune(operators, r) {
				return -1
			}
			return r
		}, tk)
		if tk != "" {
			out = append(out, tk)
		}
	}
	return strings.Join(out, " ")
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (f *ListFilter) orderClause() string {
	if f.Sort == "newest" {
		dir := "DESC"
		if f.Order == "asc" {
			dir = "ASC"
		}
		return " ORDER BY e.id " + dir
	}
	dir := "ASC"
	if f.Order == "desc" {
		dir = "DESC"
	}
	return " ORDER BY t.name " + dir
}

func uint16s(ids []uint16) []any {
	if len(ids) == 0 {
		return nil
	}
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func stringsToAny(ss []string) []any {
	if len(ss) == 0 {
		return nil
	}
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

const mediaURLPrefix = "/media/"

// mediaURL turns a stored relative path into a servable URL.
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
