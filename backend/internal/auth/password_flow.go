package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Service) userByID(ctx context.Context, id uint32) (*userRecord, error) {
	const query = `
		SELECT id, email, password_hash, role, email_verified_at, blocked_at
		FROM users WHERE id = ?`

	var u userRecord
	err := s.db.GetContext(ctx, &u, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UnverifiedUserByEmail returns the user brief when the email is registered
// and not yet verified; nil otherwise (resend-verification, uniform response).
func (s *Service) UnverifiedUserByEmail(ctx context.Context, email string) (*UserBrief, error) {
	u, err := s.userByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if u.EmailVerifiedAt != nil {
		return nil, nil
	}
	brief := u.brief()
	return &brief, nil
}

// ChangePassword verifies the current password, sets the new one and
// revokes every session except the current one (docs/AUTH.md, п.6).
func (s *Service) ChangePassword(ctx context.Context, userID uint32, currentSID uint64, currentPassword, newPassword string) error {
	u, err := s.userByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return err
	}

	if !CheckPassword(u.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	if len(newPassword) < 8 || len(newPassword) > 72 {
		return ErrInvalidPassword
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	const query = `UPDATE users SET password_hash = ? WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, query, hash, userID); err != nil {
		return err
	}
	return s.RevokeAllExcept(ctx, userID, currentSID)
}

// UserEmailByID is used by the forgot-password handler to send the reset email.
func (s *Service) UserEmailByID(ctx context.Context, userID uint32) (string, error) {
	u, err := s.userByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.Email, nil
}

// UserBriefByEmail returns the user brief for a registered email, or nil
// when the email is unknown (forgot-password must not reveal it).
func (s *Service) UserBriefByEmail(ctx context.Context, email string) (*UserBrief, error) {
	u, err := s.userByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	brief := u.brief()
	return &brief, nil
}

// ResetPassword atomically consumes the one-time password_reset token,
// sets the new password and revokes ALL sessions of the user
// (docs/AUTH.md, п.6–7).
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, err := s.ConsumeEmailToken(ctx, token, TokenPasswordReset)
	if err != nil {
		return err
	}
	if len(newPassword) < 8 || len(newPassword) > 72 {
		return ErrInvalidPassword
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	const updateQuery = `UPDATE users SET password_hash = ? WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, updateQuery, hash, userID); err != nil {
		return err
	}

	const revokeQuery = `
		UPDATE login_sessions SET revoked_at = ?
		WHERE user_id = ? AND revoked_at IS NULL`

	_, err = s.db.ExecContext(ctx, revokeQuery, time.Now().UTC(), userID)
	return err
}
