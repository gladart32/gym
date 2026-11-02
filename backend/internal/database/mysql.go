package database

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/gladart32/gym/internal/config"
)

// NewMySQL opens a connection pool and pings the database.
func NewMySQL(cfg config.MySQL) (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}
