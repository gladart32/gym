package profile

import (
	"context"
	"errors"
)

// EquipmentItem is a row of the equipment catalog. The user's inventory
// references these ids, same as exercises.equipment_id.
type EquipmentItem struct {
	ID     uint16 `json:"id" db:"id"`
	Code   string `json:"code" db:"code"`
	NameEn string `json:"name_en" db:"name_en"`
	NameRu string `json:"name_ru" db:"name_ru"`
}

// EquipmentSelection is the catalog plus the ids selected by the user,
// so the client can render the choice with a single request.
type EquipmentSelection struct {
	Items       []EquipmentItem `json:"items"`
	SelectedIDs []uint16        `json:"selected_ids"`
}

var ErrUnknownEquipment = errors.New("equipment_ids contains unknown catalog ids")

const (
	listEquipmentQuery = `
		SELECT id, code, name_en, name_ru
		FROM equipment
		ORDER BY id`

	selectedEquipmentIDsQuery = `
		SELECT equipment_id
		FROM user_equipment
		WHERE user_id = ?
		ORDER BY equipment_id`
)

// Equipment returns the full catalog and the user's selected ids.
func (s *Service) Equipment(ctx context.Context, userID uint32) (*EquipmentSelection, error) {
	var items []EquipmentItem
	if err := s.db.SelectContext(ctx, &items, listEquipmentQuery); err != nil {
		return nil, err
	}
	if items == nil {
		items = []EquipmentItem{}
	}

	var selected []uint16
	if err := s.db.SelectContext(ctx, &selected, selectedEquipmentIDsQuery, userID); err != nil {
		return nil, err
	}
	if selected == nil {
		selected = []uint16{}
	}

	return &EquipmentSelection{Items: items, SelectedIDs: selected}, nil
}

const (
	replaceDeleteQuery  = `DELETE FROM user_equipment WHERE user_id = ?`
	replaceInsertQuery  = `INSERT INTO user_equipment (user_id, equipment_id) VALUES (?, ?)`
	checkEquipmentQuery = `SELECT 1 FROM equipment WHERE id = ?`
)

// ReplaceEquipment swaps the user's inventory for the given set in one
// transaction, validating every id against the catalog.
func (s *Service) ReplaceEquipment(ctx context.Context, userID uint32, ids []uint16) ([]uint16, error) {
	seen := make(map[uint16]struct{}, len(ids))
	unique := make([]uint16, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // safe: tx is either committed or already rolled back

	for _, id := range unique {
		var one int
		if err := tx.GetContext(ctx, &one, checkEquipmentQuery, id); err != nil {
			if isNoRows(err) {
				return nil, ErrUnknownEquipment
			}
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, replaceDeleteQuery, userID); err != nil {
		return nil, err
	}
	for _, id := range unique {
		if _, err := tx.ExecContext(ctx, replaceInsertQuery, userID, id); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return unique, nil
}
