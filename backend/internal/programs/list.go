package programs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Item is one program card in the list response.
type Item struct {
	ID             uint32    `json:"id"`
	OwnerID        *uint32   `json:"owner_id"`
	CopiedFromID   *uint32   `json:"copied_from_id"`
	Name           string    `json:"name"`
	Description    *string   `json:"description"`
	AuthorName     *string   `json:"author_name"`
	Level          *string   `json:"level"`
	IsPublished    bool      `json:"is_published"`
	DaysCount      int       `json:"days_count"`
	ExercisesCount int       `json:"exercises_count"`
	IsMine         bool      `json:"is_mine"`
	IsReady        bool      `json:"is_ready"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	Q       string
	Scope   string // "" | mine | ready
	Level   string // "" | beginner | intermediate | advanced
	Sort    string // newest | name
	Order   string // asc | desc
	Page    int
	PerPage int
	UserID  uint32
	IsAdmin bool
}

const (
	DefaultPerPage = 20
	MaxPerPage     = 50
)

const listSelectQuery = `
	SELECT
		p.id,
		p.owner_id,
		p.copied_from_id,
		p.name,
		p.description,
		p.author_name,
		p.level,
		p.is_published,
		p.created_at,
		p.updated_at,
		(SELECT COUNT(*) FROM program_days pd WHERE pd.program_id = p.id) AS days_count,
		(SELECT COUNT(*) FROM program_exercises pe
		 JOIN program_days pd2 ON pd2.id = pe.program_day_id
		 WHERE pd2.program_id = p.id) AS exercises_count
	FROM programs p
	WHERE %[1]s`

const listCountQuery = `
	SELECT COUNT(*)
	FROM programs p
	WHERE %[1]s`

type listItemRow struct {
	ID             uint32    `db:"id"`
	OwnerID        *uint32   `db:"owner_id"`
	CopiedFromID   *uint32   `db:"copied_from_id"`
	Name           string    `db:"name"`
	Description    *string   `db:"description"`
	AuthorName     *string   `db:"author_name"`
	Level          *string   `db:"level"`
	IsPublished    bool      `db:"is_published"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	DaysCount      int       `db:"days_count"`
	ExercisesCount int       `db:"exercises_count"`
}

// List returns a page of programs visible to the user:
// ready published ones plus their own personal programs.
func (s *Service) List(ctx context.Context, f *ListFilter) (*ListResult, error) {
	f.Page = max(f.Page, 1)
	f.PerPage = min(max(f.PerPage, 1), MaxPerPage)

	cond, args := f.visibilityCondition()

	if searchCond, searchArg, ok := f.searchCondition(); ok {
		cond += searchCond
		args = append(args, searchArg)
	}
	if f.Level != "" {
		cond += " AND p.level = ?"
		args = append(args, f.Level)
	}

	var total int64
	if err := s.db.GetContext(ctx, &total, fmt.Sprintf(listCountQuery, cond), args...); err != nil {
		return nil, err
	}

	offset := (f.Page - 1) * f.PerPage
	args = append(args, f.PerPage, offset)

	query := fmt.Sprintf(listSelectQuery, cond) + f.orderClause() + " LIMIT ? OFFSET ?"

	var rows []listItemRow
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(rows))
	for _, r := range rows {
		items = append(items, Item{
			ID:             r.ID,
			OwnerID:        r.OwnerID,
			CopiedFromID:   r.CopiedFromID,
			Name:           r.Name,
			Description:    r.Description,
			AuthorName:     r.AuthorName,
			Level:          r.Level,
			IsPublished:    r.IsPublished,
			DaysCount:      r.DaysCount,
			ExercisesCount: r.ExercisesCount,
			IsMine:         r.OwnerID != nil && *r.OwnerID == f.UserID,
			IsReady:        r.OwnerID == nil,
			CreatedAt:      r.CreatedAt,
			UpdatedAt:      r.UpdatedAt,
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

// visibilityCondition builds the WHERE body (without leading AND):
// own personal programs always, ready programs when published —
// admins also see ready drafts.
func (f *ListFilter) visibilityCondition() (string, []any) {
	switch f.Scope {
	case "mine":
		return "p.owner_id = ?", []any{f.UserID}
	case "ready":
		if f.IsAdmin {
			return "p.owner_id IS NULL", nil
		}
		return "p.owner_id IS NULL AND p.is_published = 1", nil
	default:
		if f.IsAdmin {
			return "(p.owner_id = ? OR p.owner_id IS NULL)", []any{f.UserID}
		}
		return "(p.owner_id = ? OR (p.owner_id IS NULL AND p.is_published = 1))", []any{f.UserID}
	}
}

func (f *ListFilter) searchCondition() (string, string, bool) {
	q := strings.TrimSpace(f.Q)
	if q == "" {
		return "", "", false
	}
	return " AND p.name LIKE ?", "%" + escapeLike(q) + "%", true
}

func (f *ListFilter) orderClause() string {
	if f.Sort == "name" {
		dir := "ASC"
		if f.Order == "desc" {
			dir = "DESC"
		}
		return " ORDER BY p.name " + dir
	}
	dir := "DESC"
	if f.Order == "asc" {
		dir = "ASC"
	}
	return " ORDER BY p.id " + dir
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
