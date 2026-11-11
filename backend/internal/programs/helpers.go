package programs

import (
	"database/sql"
	"errors"
	"strings"
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
