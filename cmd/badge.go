package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/badge"
	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var badgeCmd = &cobra.Command{
	Use:   "badge <name>",
	Short: "Generate an SVG status badge for a monitor",
	Long: `Generate an SVG status badge for a monitor.

The badge shows the monitor name and its current status with colors:
  - Green (OK) - Running on schedule
  - Yellow (LATE) - Ping is overdue
  - Red (DOWN) - Grace period exceeded

Examples:
  cron-health badge daily-backup > badge.svg
  cron-health badge daily-backup | cat`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		m, err := database.GetMonitorByName(name)
		if err != nil || m == nil {
			// Generate a gray "not found" badge
			fmt.Print(badge.Generate(name, "unknown"))
			return nil
		}

		status := monitor.CalculateStatus(m)
		fmt.Print(badge.Generate(m.Name, status))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(badgeCmd)
}
