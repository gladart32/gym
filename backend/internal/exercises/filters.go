package exercises

import (
	"context"
	"fmt"
)

// TrackingTypeRef is a tracking_type enum value with a localized label.
type TrackingTypeRef struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

// Filters is everything the client needs to render the filter panel.
type Filters struct {
	BodyParts     []Ref            `json:"body_parts"`
	Equipment     []Ref            `json:"equipment"`
	Muscles       []Ref            `json:"muscles"`
	TrackingTypes []TrackingTypeRef `json:"tracking_types"`
}

const (
	filtersBodyPartsQueryTmpl = `SELECT id, %[1]s AS name FROM body_parts ORDER BY name`
	filtersEquipmentQueryTmpl = `SELECT id, %[1]s AS name FROM equipment ORDER BY name`
	filtersMusclesQueryTmpl   = `SELECT id, %[1]s AS name FROM muscles ORDER BY name`
)

// trackingTypesOrder keeps a stable enum order across locales.
var trackingTypesOrder = []string{"weight_reps", "reps", "time", "distance_time"}

var trackingTypeNames = map[string]map[string]string{
	LangRu: {
		"weight_reps":   "Вес × повторы",
		"reps":          "Повторы",
		"time":          "Время",
		"distance_time": "Дистанция + время",
	},
	LangEn: {
		"weight_reps":   "Weight × reps",
		"reps":          "Reps",
		"time":          "Time",
		"distance_time": "Distance + time",
	},
}

// IsTrackingType reports whether the value belongs to the tracking_type enum.
func IsTrackingType(v string) bool {
	_, ok := trackingTypeNames[LangRu][v]
	return ok
}

// Filters returns the localized filter dictionaries.
func (s *Service) Filters(ctx context.Context, lang string) (*Filters, error) {
	col := nameColumn(lang)

	var bodyParts, equipment, muscles []Ref
	if err := s.db.SelectContext(ctx, &bodyParts, fmt.Sprintf(filtersBodyPartsQueryTmpl, col)); err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &equipment, fmt.Sprintf(filtersEquipmentQueryTmpl, col)); err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &muscles, fmt.Sprintf(filtersMusclesQueryTmpl, col)); err != nil {
		return nil, err
	}

	names := trackingTypeNames[lang]
	if names == nil {
		names = trackingTypeNames[LangRu]
	}
	trackingTypes := make([]TrackingTypeRef, 0, len(trackingTypesOrder))
	for _, v := range trackingTypesOrder {
		trackingTypes = append(trackingTypes, TrackingTypeRef{Value: v, Name: names[v]})
	}

	return &Filters{
		BodyParts:     nonNil(bodyParts),
		Equipment:     nonNil(equipment),
		Muscles:       nonNil(muscles),
		TrackingTypes: trackingTypes,
	}, nil
}

func nonNil(refs []Ref) []Ref {
	if refs == nil {
		return []Ref{}
	}
	return refs
}
