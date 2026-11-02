// Package testutil provides helpers for integration tests:
// it loads config/test.yaml, applies goose migrations and builds a full App
// via the same manual DI as production code.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"github.com/gladart32/gym/internal/app"
	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/migrations"
)

// Start boots a full application against the test infrastructure
// (mysql-test :3307, redis-test :6380, see docker-compose profile "test").
// Migrations are applied before the app is created; resources are released
// via t.Cleanup.
func Start(t *testing.T) *app.App {
	t.Helper()

	cfg := LoadConfig(t)
	MigrateUp(t, cfg)

	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("init app: %v", err)
	}
	t.Cleanup(a.Close)

	return a
}

// LoadConfig loads config/test.yaml regardless of the current working directory.
func LoadConfig(t *testing.T) *config.Config {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine testutil path")
	}
	cfgPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "test.yaml")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

// MigrateUp applies all pending migrations to the test database.
func MigrateUp(t *testing.T, cfg *config.Config) {
	t.Helper()

	db, err := sql.Open("mysql", cfg.MySQL.DSN())
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectMySQL, db, migrations.FS)
	if err != nil {
		t.Fatalf("create goose provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}
