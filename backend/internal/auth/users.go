package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/gladart32/gym/internal/profile"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
	ErrEmailNotVerified   = errors.New("email is not verified")
	ErrBlocked            = errors.New("user is blocked")
	ErrInvalidPassword    = errors.New("password must be between 8 and 72 characters")
	ErrTokenInvalid       = errors.New("invalid or expired token")
)

type UserBrief struct {
	ID    uint32 `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type userRecord struct {
	ID              uint32     `db:"id"`
	Email           string     `db:"email"`
	PasswordHash    string     `db:"password_hash"`
	Role            string     `db:"role"`
	EmailVerifiedAt *time.Time `db:"email_verified_at"`
	BlockedAt       *time.Time `db:"blocked_at"`
}

func (u *userRecord) brief() UserBrief {
	return UserBrief{ID: u.ID, Email: u.Email, Role: u.Role}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Register creates an unverified user and returns its brief.
// timezone (optional, IANA name) is stored for schedule reminders; an empty
// value keeps the column default.
func (s *Service) Register(ctx context.Context, email, password, name, timezone string) (*UserBrief, error) {
	email = normalizeEmail(email)
	if len(password) < 8 || len(password) > 72 {
		return nil, ErrInvalidPassword
	}
	name = truncate(strings.TrimSpace(name), maxUserName)

	loc, err := profile.NormalizeTimezone(timezone)
	if err != nil {
		return nil, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	if loc == nil {
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`, email, hash, name)
		if err != nil {
			return nil, registerError(err)
		}
		return insertUserBrief(res, email)
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, name, timezone) VALUES (?, ?, ?, ?)`,
		email, hash, name, profile.TimezoneName(loc))
	if err != nil {
		return nil, registerError(err)
	}
	return insertUserBrief(res, email)
}

func registerError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrEmailTaken
	}
	return err
}

func insertUserBrief(res sql.Result, email string) (*UserBrief, error) {
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &UserBrief{ID: uint32(id), Email: email, Role: RoleUser}, nil
}

func (s *Service) userByEmail(ctx context.Context, email string) (*userRecord, error) {
	const query = `
		SELECT id, email, password_hash, role, email_verified_at, blocked_at
		FROM users WHERE email = ?`

	var u userRecord
	err := s.db.GetContext(ctx, &u, query, normalizeEmail(email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
