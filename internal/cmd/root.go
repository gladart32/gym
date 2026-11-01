package cmd

import (
	"github.com/spf13/cobra"
)

var configPath string

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "gym",
		Short:        "Gym API server and background worker",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&configPath, "config", "",
		"path to config file (default: config/local.yaml)")

	root.AddCommand(
		newServeCommand(),
		newWorkerCommand(),
		newMigrateCommand(),
	)

	return root
}

func Execute() error {
	return NewRootCommand().Execute()
}
