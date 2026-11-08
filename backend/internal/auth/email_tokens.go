package auth

import (
	"context"
	"time"
)

const (
	TokenEmailVerify   = "email_verify"
	TokenPasswordReset = "password_reset"
)

// CreateEmailToken invalidates the previous unused token of the same type
// and issues a new one-time token; only its hash is stored (docs/AUTH.md, п.7).
func (s *Service) CreateEmailToken(ctx context.Context, userID uint32, tokenType string) (string, error) {
	const invalidateQuery = `
		UPDATE email_action_tokens SET used_at = ?
		WHERE user_id = ? AND type = ? AND used_at IS NULL`

	if _, err := s.db.ExecContext(ctx, invalidateQuery, time.Now().UTC(), userID, tokenType); err != nil {
		return "", err
	}

	token, hash, err := NewToken()
	if err != nil {
		return "", err
	}

	ttl := s.cfg.PasswordResetTTL
	if tokenType == TokenEmailVerify {
		ttl = s.cfg.EmailVerifyTTL
	}

	const insertQuery = `
		INSERT INTO email_action_tokens (user_id, type, token_hash, expires_at)
		VALUES (?, ?, ?, ?)`

	_, err = s.db.ExecContext(ctx, insertQuery, userID, tokenType, hash, time.Now().UTC().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeEmailToken atomically marks a one-time email token as used.
// A single UPDATE guards against a double click; the winner gets the user id.
func (s *Service) ConsumeEmailToken(ctx context.Context, token, tokenType string) (uint32, error) {
	hash := HashToken(token)

	const consumeQuery = `
		UPDATE email_action_tokens SET used_at = ?
		WHERE token_hash = ? AND type = ? AND used_at IS NULL AND expires_at > ?`

	res, err := s.db.ExecContext(ctx, consumeQuery, time.Now().UTC(), hash, tokenType, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return 0, ErrTokenInvalid
	}

	const userQuery = `SELECT user_id FROM email_action_tokens WHERE token_hash = ?`
	var userID uint32
	if err := s.db.GetContext(ctx, &userID, userQuery, hash); err != nil {
		return 0, err
	}
	return userID, nil
}

// VerifyEmail consumes a one-time email_verify token and marks the email
// as verified. It is idempotent for an already-verified user.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.ConsumeEmailToken(ctx, token, TokenEmailVerify)
	if err != nil {
		return err
	}

	const query = `
		UPDATE users SET email_verified_at = ?
		WHERE id = ? AND email_verified_at IS NULL`

	_, err = s.db.ExecContext(ctx, query, time.Now().UTC(), userID)
	return err
}
