// Package profile provides the user profile domain: static profile fields,
// body measurement history and selected equipment (sqlx, raw SQL).
package profile

import (
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

// Service is the profile repository and business logic in one place,
// mirroring the auth domain layout.
type Service struct {
	db  *sqlx.DB
	log *slog.Logger
}

func NewService(db *sqlx.DB, log *slog.Logger) *Service {
	return &Service{db: db, log: log}
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
