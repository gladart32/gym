package profile

import (
	"errors"
	"strings"
	"time"
)

// maxTimezoneLen matches the users.timezone column width.
const maxTimezoneLen = 64

// ErrInvalidTimezone is not an IANA timezone name (or too long).
var ErrInvalidTimezone = errors.New("timezone must be a valid IANA name (e.g. Europe/Moscow)")

// NormalizeTimezone validates an IANA timezone name and returns its location.
// An empty (after trim) name means "not provided": nil location, nil error.
func NormalizeTimezone(tz string) (*time.Location, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return nil, nil
	}
	if len(tz) > maxTimezoneLen {
		return nil, ErrInvalidTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, ErrInvalidTimezone
	}
	return loc, nil
}

// TimezoneName returns the canonical IANA name of the location.
func TimezoneName(loc *time.Location) string {
	if loc == nil {
		return ""
	}
	return loc.String()
}
