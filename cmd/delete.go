package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a monitor",
	Long:  `Delete a monitor and all its ping history`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Check if monitor exists
		m, err := database.GetMonitorByName(name)
		if err != nil || m == nil {
			return fmt.Errorf("monitor '%s' not found", name)
		}

		if err := database.DeleteMonitor(name); err != nil {
			return fmt.Errorf("failed to delete monitor: %w", err)
		}

		color.Green("✓ Deleted monitor '%s'", name)
		return nil
	},
}
