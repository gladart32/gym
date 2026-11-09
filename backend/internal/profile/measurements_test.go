package profile

import (
	"errors"
	"testing"
	"time"
)

func f32(v float64) *float64 { return &v }

// mskMidnight returns midnight of today's calendar day in a fixed UTC+3 zone —
// exactly what the handler produces for a client east of UTC before 03:00 local.
func mskMidnight() time.Time {
	msk := time.FixedZone("MSK", 3*60*60)
	y, mth, d := time.Now().In(msk).Date()
	return time.Date(y, mth, d, 0, 0, 0, 0, msk)
}

func TestMeasurementValidateMeasuredOn(t *testing.T) {
	msk := mskMidnight()
	utcY, utcMth, utcD := time.Now().UTC().Date()
	midnightUTC := time.Date(utcY, utcMth, utcD, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		ts   time.Time
		want error
	}{
		{"today client midnight UTC+3", msk, nil},
		{"today midnight UTC", midnightUTC, nil},
		{"tomorrow client UTC+3", msk.Add(24 * time.Hour), ErrInvalidMeasuredOn},
		{"tomorrow UTC", midnightUTC.Add(24 * time.Hour), ErrInvalidMeasuredOn},
		{"yesterday", msk.Add(-24 * time.Hour), nil},
		{"zero", time.Time{}, ErrInvalidMeasuredOn},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Measurement{MeasuredOn: tc.ts, WeightKg: f32(60)}
			if got := m.validate(); !errors.Is(got, tc.want) {
				t.Fatalf("validate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMeasurementValidateFields(t *testing.T) {
	validDate := mskMidnight()

	cases := []struct {
		name string
		m    *Measurement
		want error
	}{
		{"weight below range", &Measurement{MeasuredOn: validDate, WeightKg: f32(19)}, ErrInvalidWeight},
		{"weight above range", &Measurement{MeasuredOn: validDate, WeightKg: f32(501)}, ErrInvalidWeight},
		{"height below range", &Measurement{MeasuredOn: validDate, HeightCm: f32(49)}, ErrInvalidHeight},
		{"height above range", &Measurement{MeasuredOn: validDate, HeightCm: f32(301)}, ErrInvalidHeight},
		{"empty", &Measurement{MeasuredOn: validDate}, ErrEmptyMeasurement},
		{"valid", &Measurement{MeasuredOn: validDate, WeightKg: f32(60), HeightCm: f32(175)}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.validate(); !errors.Is(got, tc.want) {
				t.Fatalf("validate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestToDateKeepsCalendarDate(t *testing.T) {
	// Midnight Nov 10 in UTC+3 = Nov 9 21:00 UTC; the stored DATE
	// must still be Nov 10, not Nov 9.
	msk := mskMidnight()

	got := toDate(&msk)
	want := time.Date(msk.Year(), msk.Month(), msk.Day(), 0, 0, 0, 0, time.UTC)

	if got.Location() != time.UTC || !got.Equal(want) {
		t.Fatalf("toDate() = %v (%s), want %v UTC", got, got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
	if got.UTC().Format("2006-01-02") != msk.Format("2006-01-02") {
		t.Fatalf("calendar date shifted: got %s, want %s",
			got.UTC().Format("2006-01-02"), msk.Format("2006-01-02"))
	}
}
