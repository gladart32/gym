package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gladart32/gym/internal/config"
)

var envFlag string
var configPath string

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "gym",
		Short:        "Gym API server and background worker",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&envFlag, "env", "",
		"environment: dev | prod | test (overrides GYM_ENV)")
	root.PersistentFlags().StringVar(&configPath, "config", "",
		"path to config file (default: config/<env>.yaml)")

	root.AddCommand(
		newServeCommand(),
		newWorkerCommand(),
		newMigrateCommand(),
		newCreateAdminCommand(),
	)

	return root
}

func Execute() error {
	return NewRootCommand().Execute()
}

// resolveEnv: --env flag > GYM_ENV env var > fail-fast error.
func resolveEnv() (string, error) {
	env := envFlag
	if env == "" {
		env = os.Getenv("GYM_ENV")
	}
	if err := config.ValidateEnv(env); err != nil {
		return "", err
	}
	return env, nil
}

func resolveConfigPath(env string) string {
	if configPath != "" {
		return configPath
	}
	return filepath.Join("config", env+".yaml")
}
