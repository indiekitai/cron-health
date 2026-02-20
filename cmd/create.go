package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/cron"
	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var (
	createInterval string
	createGrace    string
	createCron     string
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new monitor",
	Long: `Create a new monitor with the specified name and expected interval.

You can use either --interval for fixed intervals or --cron for cron expressions.

Examples:
  cron-health create daily-backup --interval 24h --grace 1h
  cron-health create hourly-sync --interval 1h --grace 5m
  cron-health create weekly-report --interval 7d
  cron-health create nightly-backup --cron "0 2 * * *" --grace 1h
  cron-health create monday-report --cron "0 9 * * 1"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Either interval or cron is required
		if createInterval == "" && createCron == "" {
			return fmt.Errorf("either --interval or --cron is required")
		}

		// Parse grace period
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

		var m *db.Monitor
		var interval time.Duration

		if createCron != "" {
			// Cron-based monitor
			parser := cron.New()
			if err := parser.Validate(createCron); err != nil {
				return fmt.Errorf("invalid cron expression: %w", err)
			}

			nextRun, err := parser.NextRunNow(createCron)
			if err != nil {
				return fmt.Errorf("failed to calculate next run: %w", err)
			}

			// If interval is also provided, use it; otherwise default to 24h
			var intervalSeconds int64 = 24 * 60 * 60
			if createInterval != "" {
				interval, err = monitor.ParseDuration(createInterval)
				if err != nil {
					return fmt.Errorf("invalid interval: %w", err)
				}
				intervalSeconds = int64(interval.Seconds())
			}

			m, err = database.CreateMonitorWithCron(name, intervalSeconds, grace, createCron, &nextRun)
			if err != nil {
				return fmt.Errorf("failed to create monitor: %w", err)
			}

			color.Green("✓ Created monitor '%s' (cron-based)", m.Name)
			fmt.Printf("  Cron:     %s\n", createCron)
			fmt.Printf("  Next run: %s\n", nextRun.Format("2006-01-02 15:04:05"))
		} else {
			// Interval-based monitor
			interval, err = monitor.ParseDuration(createInterval)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}

			m, err = database.CreateMonitor(name, int64(interval.Seconds()), grace)
			if err != nil {
				return fmt.Errorf("failed to create monitor: %w", err)
			}

			color.Green("✓ Created monitor '%s'", m.Name)
			fmt.Printf("  Interval: %s\n", monitor.FormatDuration(interval))
		}

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
	createCmd.Flags().StringVarP(&createCron, "cron", "c", "", "Cron expression (e.g., \"0 2 * * *\" for 2am daily)")
}
