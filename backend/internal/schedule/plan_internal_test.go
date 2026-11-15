package schedule

import (
	"testing"
	"time"
)

// todayIn must keep the calendar date of the given location: for zones far
// from UTC the local "today" differs from the UTC one near midnight.
func TestTodayInFarTimezones(t *testing.T) {
	for _, loc := range []*time.Location{
		mustLoc(t, "Pacific/Kiritimati"), // UTC+14
		mustLoc(t, "Pacific/Pago_Pago"),  // UTC-11
		mustLoc(t, "Europe/Moscow"),      // UTC+3
		time.UTC,
	} {
		now := time.Now().In(loc)
		want := now.Format("2006-01-02")
		if got := todayIn(loc).Format("2006-01-02"); got != want {
			t.Fatalf("%s: todayIn=%s, local date=%s", loc, got, want)
		}
	}
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}
