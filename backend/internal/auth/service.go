package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/gladart32/gym/internal/config"
)

// Service wires the auth repository (sqlx, raw SQL) and token helpers.
type Service struct {
	db        *sqlx.DB
	cfg       config.Auth
	jwt       *JWTManager
	log       *slog.Logger
	dummyHash string
}

func NewService(db *sqlx.DB, cfg config.Auth, log *slog.Logger) *Service {
	dummyHash, _ := HashPassword("timing-equalizer-dummy-password")
	return &Service{db: db, cfg: cfg, jwt: NewJWTManager(cfg.JWTSecret, cfg.AccessTTL), log: log, dummyHash: dummyHash}
}

func (s *Service) JWT() *JWTManager {
	return s.jwt
}

func (s *Service) Config() config.Auth {
	return s.cfg
}

// SessionActive reports whether the session with the given id belongs to
// userID and is neither revoked nor expired (docs/AUTH.md, п.2, шаг 2).
func (s *Service) SessionActive(ctx context.Context, sid uint64, userID uint32) (bool, error) {
	const query = `
		SELECT 1 FROM login_sessions
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP()`

	var one int
	err := s.db.GetContext(ctx, &one, query, sid, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
