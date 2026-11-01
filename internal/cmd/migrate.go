package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	"github.com/gladart32/gym/migrations"
)

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migrations (goose)",
	}
	cmd.AddCommand(
		newMigrateUpCommand(),
		newMigrateDownCommand(),
		newMigrateStatusCommand(),
		newMigrateCreateCommand(),
	)
	return cmd
}

func withGooseProvider(run func(ctx context.Context, p *goose.Provider) error) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.MySQL.DSN())
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectMySQL, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return run(ctx, provider)
}

func newMigrateUpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGooseProvider(func(ctx context.Context, p *goose.Provider) error {
				results, err := p.Up(ctx)
				if err != nil {
					return err
				}
				if len(results) == 0 {
					fmt.Println("no new migrations")
					return nil
				}
				for _, r := range results {
					fmt.Printf("applied  %s\n", r.Source.Path)
				}
				return nil
			})
		},
	}
}

func newMigrateDownCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back the last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGooseProvider(func(ctx context.Context, p *goose.Provider) error {
				result, err := p.Down(ctx)
				if err != nil {
					return err
				}
				fmt.Printf("rolled back  %s\n", result.Source.Path)
				return nil
			})
		},
	}
}

func newMigrateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show migration statuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGooseProvider(func(ctx context.Context, p *goose.Provider) error {
				statuses, err := p.Status(ctx)
				if err != nil {
					return err
				}
				fmt.Printf("%-16s %-10s %s\n", "VERSION", "STATE", "SOURCE")
				for _, s := range statuses {
					fmt.Printf("%-16d %-10s %s\n", s.Source.Version, s.State, s.Source.Path)
				}
				return nil
			})
		},
	}
}

var migrateNameRe = regexp.MustCompile(`[^a-z0-9_]+`)

func newMigrateCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create NAME",
		Short: "Create a new migration file in ./migrations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.ToLower(args[0])
			name = strings.ReplaceAll(name, " ", "_")
			name = strings.Trim(migrateNameRe.ReplaceAllString(name, "_"), "_")

			dir := "migrations"
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}

			version := time.Now().UTC().Format("20060102150405")
			path := filepath.Join(dir, version+"_"+name+".sql")

			content := "-- +goose Up\n\n-- +goose Down\n"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return err
			}

			fmt.Println("created", path)
			return nil
		},
	}
}
