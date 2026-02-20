package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var (
	createInterval string
	createGrace    string
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new monitor",
	Long: `Create a new monitor with the specified name and expected interval.

Examples:
  cron-health create daily-backup --interval 24h --grace 1h
  cron-health create hourly-sync --interval 1h --grace 5m
  cron-health create weekly-report --interval 7d`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if createInterval == "" {
			return fmt.Errorf("--interval is required")
		}

		interval, err := monitor.ParseDuration(createInterval)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}

		var grace int64 = 0
		if createGrace != "" {
			graceDur, err := monitor.ParseDuration(createGrace)
			if err != nil {
				return fmt.Errorf("invalid grace period: %w", err)
			}
			grace = int64(graceDur.Seconds())
		}

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Check if monitor already exists
		existing, _ := database.GetMonitorByName(name)
		if existing != nil {
			return fmt.Errorf("monitor '%s' already exists", name)
		}

		m, err := database.CreateMonitor(name, int64(interval.Seconds()), grace)
		if err != nil {
			return fmt.Errorf("failed to create monitor: %w", err)
		}

		color.Green("✓ Created monitor '%s'", m.Name)
		fmt.Printf("  Interval: %s\n", monitor.FormatDuration(interval))
		if grace > 0 {
			graceDuration := time.Duration(grace) * time.Second
			fmt.Printf("  Grace:    %s\n", monitor.FormatDuration(graceDuration))
		}
		fmt.Println()
		fmt.Printf("Ping endpoint: GET /ping/%s\n", name)

		return nil
	},
}

func init() {
	createCmd.Flags().StringVarP(&createInterval, "interval", "i", "", "Expected interval between pings (e.g., 1h, 30m, 1d)")
	createCmd.Flags().StringVarP(&createGrace, "grace", "g", "", "Grace period before marking as DOWN (e.g., 5m, 1h)")
	createCmd.MarkFlagRequired("interval")
}
