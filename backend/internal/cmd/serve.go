package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gladart32/gym/internal/app"
	"github.com/gladart32/gym/internal/config"
)

func loadConfig() (*config.Config, error) {
	env, err := resolveEnv()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(resolveConfigPath(env))
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func commandContext() (context.Context, func()) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}
			defer application.Close()

			ctx, stop := commandContext()
			defer stop()

			return application.HTTPServer().Run(ctx)
		},
	}
}

func newWorkerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Start the background task worker (asynq server + scheduler)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}
			defer application.Close()

			ctx, stop := commandContext()
			defer stop()

			return application.TaskWorker().Run(ctx)
		},
	}
}
