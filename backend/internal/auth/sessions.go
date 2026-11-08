package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	maxUserName    = 100
	maxDeviceName  = 100
	maxOS          = 50
	maxAppVersion  = 20
	maxUserAgent   = 512
)

type LoginInput struct {
	Email       string
	Password    string
	ClientType  string
	DeviceName  string
	OS          string
	AppVersion  string
	UserAgent   string
	IP          []byte
}

type IssuedTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	SID          uint64
	ClientType   string
	User         UserBrief
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// nullable turns an empty string into SQL NULL and truncates to the column limit.
func nullable(s string, n int) any {
	if s == "" {
		return nil
	}
	return truncate(s, n)
}

// Login verifies credentials, creates a login session and issues a
// token pair (docs/AUTH.md, п.1).
func (s *Service) Login(ctx context.Context, in LoginInput) (*IssuedTokens, error) {
	u, err := s.userByEmail(ctx, in.Email)
	if errors.Is(err, sql.ErrNoRows) {
		// keep roughly the same response time as the bcrypt path
		CheckPassword(s.dummyHash, in.Password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if !CheckPassword(u.PasswordHash, in.Password) {
		return nil, ErrInvalidCredentials
	}
	if u.BlockedAt != nil {
		return nil, ErrBlocked
	}
	if u.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}

	sid, refresh, err := s.createSession(ctx, u.ID, in)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(u.brief(), sid, in.ClientType, refresh)
}

func (s *Service) createSession(ctx context.Context, userID uint32, in LoginInput) (uint64, string, error) {
	refresh, hash, err := NewToken()
	if err != nil {
		return 0, "", err
	}

	const query = `
		INSERT INTO login_sessions
			(user_id, client_type, refresh_token_hash, device_name, os, app_version, user_agent, ip, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := s.db.ExecContext(ctx, query,
		userID,
		in.ClientType,
		hash,
		nullable(in.DeviceName, maxDeviceName),
		nullable(in.OS, maxOS),
		nullable(in.AppVersion, maxAppVersion),
		nullable(in.UserAgent, maxUserAgent),
		in.IP,
		time.Now().UTC().Add(s.cfg.RefreshTTL),
	)
	if err != nil {
		return 0, "", err
	}
	sid, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	return uint64(sid), refresh, nil
}

func (s *Service) issueTokens(user UserBrief, sid uint64, clientType, refresh string) (*IssuedTokens, error) {
	access, err := s.jwt.IssueAccess(user.ID, sid, clientType, user.Role)
	if err != nil {
		return nil, err
	}
	return &IssuedTokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
		SID:          sid,
		ClientType:   clientType,
		User:         user,
	}, nil
}

type refreshSessionRow struct {
	SID        uint64     `db:"sid"`
	ClientType string     `db:"client_type"`
	ExpiresAt  time.Time  `db:"expires_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
	UserID     uint32     `db:"user_id"`
	Email      string     `db:"email"`
	Role       string     `db:"role"`
	BlockedAt  *time.Time `db:"blocked_at"`
}

// Refresh rotates the one-time refresh token: a new pair is issued with the
// same sid, the old hash moves to prev_refresh_token_hash and the session
// expiry slides forward (docs/AUTH.md, п.3). Presenting the previous token
// is treated as theft and revokes the session, except within the tolerance
// window right after rotation (two browser tabs).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*IssuedTokens, error) {
	hash := HashToken(refreshToken)

	const currentQuery = `
		SELECT s.id AS sid, s.client_type, s.expires_at, s.revoked_at,
		       u.id AS user_id, u.email, u.role, u.blocked_at
		FROM login_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.refresh_token_hash = ?`

	var row refreshSessionRow
	err := s.db.GetContext(ctx, &row, currentQuery, hash)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, s.handlePrevToken(ctx, hash)
	case err != nil:
		return nil, err
	}

	if row.RevokedAt != nil || !row.ExpiresAt.After(time.Now().UTC()) {
		return nil, ErrTokenInvalid
	}
	if row.BlockedAt != nil {
		_ = s.revokeSession(ctx, row.SID, row.UserID)
		return nil, ErrTokenInvalid
	}

	newRefresh, newHash, err := NewToken()
	if err != nil {
		return nil, err
	}

	const rotateQuery = `
		UPDATE login_sessions
		SET prev_refresh_token_hash = refresh_token_hash,
		    refresh_token_hash = ?,
		    last_used_at = ?,
		    expires_at = ?
		WHERE id = ? AND refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > ?`

	res, err := s.db.ExecContext(ctx, rotateQuery,
		newHash,
		time.Now().UTC(),
		time.Now().UTC().Add(s.cfg.RefreshTTL),
		row.SID,
		hash,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	// 0 rows: a parallel refresh won the rotation race.
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, ErrTokenInvalid
	}

	user := UserBrief{ID: row.UserID, Email: row.Email, Role: row.Role}
	return s.issueTokens(user, row.SID, row.ClientType, newRefresh)
}

// handlePrevToken checks whether the presented token is the previous
// (already rotated) refresh token of a live session: outside the tolerance
// window this means theft, so the session is revoked.
func (s *Service) handlePrevToken(ctx context.Context, hash []byte) error {
	const query = `
		SELECT id, user_id, revoked_at, last_used_at
		FROM login_sessions WHERE prev_refresh_token_hash = ?`

	var prev struct {
		ID         uint64     `db:"id"`
		UserID     uint32     `db:"user_id"`
		RevokedAt  *time.Time `db:"revoked_at"`
		LastUsedAt time.Time  `db:"last_used_at"`
	}
	err := s.db.GetContext(ctx, &prev, query, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTokenInvalid
	}
	if err != nil {
		return err
	}
	if prev.RevokedAt != nil {
		return ErrTokenInvalid
	}

	cutoff := time.Now().UTC().Add(-time.Duration(s.cfg.ToleranceSec) * time.Second)
	if prev.LastUsedAt.After(cutoff) {
		return ErrTokenInvalid
	}

	if err := s.revokeSession(ctx, prev.ID, prev.UserID); err != nil {
		return err
	}
	s.log.WarnContext(ctx, "refresh token reuse detected, session revoked",
		"user_id", prev.UserID, "sid", prev.ID)
	return ErrTokenInvalid
}

// RevokeSession revokes the session if it belongs to userID (logout).
func (s *Service) RevokeSession(ctx context.Context, sid uint64, userID uint32) error {
	return s.revokeSession(ctx, sid, userID)
}

func (s *Service) revokeSession(ctx context.Context, sid uint64, userID uint32) error {
	const query = `
		UPDATE login_sessions SET revoked_at = ?
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, time.Now().UTC(), sid, userID)
	return err
}

// RevokeAllExcept revokes every live session of the user except the current
// one (docs/AUTH.md, п.5).
func (s *Service) RevokeAllExcept(ctx context.Context, userID uint32, exceptSID uint64) error {
	const query = `
		UPDATE login_sessions SET revoked_at = ?
		WHERE user_id = ? AND id <> ? AND revoked_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, time.Now().UTC(), userID, exceptSID)
	return err
}

type SessionInfo struct {
	ID          uint64    `json:"id" db:"id"`
	ClientType  string    `json:"client_type" db:"client_type"`
	DeviceName  *string   `json:"device_name" db:"device_name"`
	OS          *string   `json:"os" db:"os"`
	AppVersion  *string   `json:"app_version" db:"app_version"`
	IP          *string   `json:"ip" db:"ip"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at" db:"last_used_at"`
	IsCurrent   bool      `json:"is_current" db:"is_current"`
}

// ListSessions returns the user's active sessions with is_current
// (docs/AUTH.md, п.4).
func (s *Service) ListSessions(ctx context.Context, userID uint32, currentSID uint64) ([]SessionInfo, error) {
	const query = `
		SELECT id, client_type, device_name, os, app_version,
		       INET6_NTOA(ip) AS ip, created_at, last_used_at,
		       id = ? AS is_current
		FROM login_sessions
		WHERE user_id = ? AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP()
		ORDER BY is_current DESC, last_used_at DESC`

	var sessions []SessionInfo
	if err := s.db.SelectContext(ctx, &sessions, query, currentSID, userID); err != nil {
		return nil, err
	}
	return sessions, nil
}
