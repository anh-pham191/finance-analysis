package main

import (
	"io"

	"github.com/spf13/cobra"
)

var (
	version       = "dev"
	migrationsDir = "migrations"
)

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "finance",
		Short:         "Analyse personal finance data",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.AddCommand(newCategoriseCommand(stdout, stderr))
	cmd.AddCommand(newHealthCommand(stdout))
	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newRecatCommand(stdout, stderr))
	cmd.AddCommand(newSyncCommand(stdout, stderr))
	cmd.AddCommand(newUnrecatCommand(stdout, stderr))

	return cmd
}
