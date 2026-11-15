package programs

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

const mediaURLPrefix = "/media/"

// nameColumn maps a whitelisted lang to the localized name column
// (same rule as the exercises domain).
func nameColumn(lang string) string {
	if lang == "en" {
		return "name_en"
	}
	return "name_ru"
}

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

// weekdayOrder is the canonical storage order of the MySQL SET column.
var weekdayOrder = [...]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// parseWeekdays splits a MySQL SET value ("mon,wed,fri") into a slice.
func parseWeekdays(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// joinWeekdays renders the given day codes in canonical order.
func joinWeekdays(set map[string]struct{}) string {
	out := make([]string, 0, len(set))
	for _, w := range weekdayOrder {
		if _, ok := set[w]; ok {
			out = append(out, w)
		}
	}
	return strings.Join(out, ",")
}

// formatDate renders a date as YYYY-MM-DD (nil-safe).
func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}
