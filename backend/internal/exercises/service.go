// Package exercises provides the exercise catalog domain: filtered list,
// detail card and filter dictionaries (sqlx, raw SQL).
package exercises

import (
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

// Service is the exercise catalog repository and business logic in one
// place, mirroring the profile domain layout.
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
