// Package programs provides the workout program domain: visible program
// lists, the full program tree (days -> exercises -> sets) and CRUD
// mutations with owner/admin permissions (sqlx, raw SQL).
package programs

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
)

// Service is the programs repository and business logic in one place,
// mirroring the exercises domain layout.
type Service struct {
	db  *sqlx.DB
	log *slog.Logger
}

func NewService(db *sqlx.DB, log *slog.Logger) *Service {
	return &Service{db: db, log: log}
}
