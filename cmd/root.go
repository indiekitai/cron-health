package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "1.0.0"

var rootCmd = &cobra.Command{
	Use:     "cron-health",
	Short:   "Monitor your cron jobs",
	Version: Version,
	Long: `cron-health is a CLI tool for monitoring cron jobs.
It provides a simple HTTP endpoint for your cron jobs to ping,
and alerts you when jobs fail to run on schedule.

Open source alternative to healthchecks.io`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(logsCmd)
}
