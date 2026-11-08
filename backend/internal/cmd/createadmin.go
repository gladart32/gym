package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"

	"github.com/gladart32/gym/internal/auth"
)

func newCreateAdminCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create-admin EMAIL PASSWORD",
		Short: "Create an admin user with a verified email",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := strings.ToLower(strings.TrimSpace(args[0]))
			password := args[1]

			if !strings.Contains(email, "@") || strings.ContainsAny(email, " \t") {
				return fmt.Errorf("invalid email %q", email)
			}
			if len(password) < 8 {
				return errors.New("password must be at least 8 characters")
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			db, err := sqlx.Connect("mysql", cfg.MySQL.DSN())
			if err != nil {
				return fmt.Errorf("connect mysql: %w", err)
			}
			defer db.Close()

			hash, err := auth.HashPassword(password)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}

			res, err := db.Exec(`
				INSERT INTO users (email, password_hash, role, email_verified_at)
				VALUES (?, ?, 'admin', UTC_TIMESTAMP())`, email, hash)
			if err != nil {
				var mysqlErr *mysql.MySQLError
				if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
					return fmt.Errorf("user %s already exists", email)
				}
				return fmt.Errorf("create admin: %w", err)
			}
			id, _ := res.LastInsertId()

			fmt.Printf("admin created: id=%d email=%s\n", id, email)
			return nil
		},
	}
}
