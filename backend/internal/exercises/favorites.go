package exercises

import "context"

// AddFavorite marks the exercise as favorite for the user (idempotent).
// Returns ErrNotFound when the exercise does not exist or is inactive.
func (s *Service) AddFavorite(ctx context.Context, userID, exerciseID uint32) error {
	var active bool
	err := s.db.GetContext(ctx, &active,
		`SELECT is_active FROM exercises WHERE id = ?`, exerciseID)
	if isNoRows(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !active {
		return ErrNotFound
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT IGNORE INTO exercise_favorites (user_id, exercise_id) VALUES (?, ?)`,
		userID, exerciseID)
	return err
}

// RemoveFavorite clears the favorite mark (idempotent: no error when the
// pair is already absent).
func (s *Service) RemoveFavorite(ctx context.Context, userID, exerciseID uint32) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM exercise_favorites WHERE user_id = ? AND exercise_id = ?`,
		userID, exerciseID)
	return err
}
