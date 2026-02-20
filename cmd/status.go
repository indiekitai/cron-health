package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var statusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show detailed status of a monitor or all monitors",
	Long:  `Show detailed status information for a specific monitor or all monitors`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if len(args) == 0 {
			// Show all monitors
			monitors, err := database.ListMonitors()
			if err != nil {
				return fmt.Errorf("failed to list monitors: %w", err)
			}

			if len(monitors) == 0 {
				fmt.Println("No monitors configured.")
				return nil
			}

			for i, m := range monitors {
				if i > 0 {
					fmt.Println()
				}
				printMonitorStatus(m)
			}
			return nil
		}

		// Show specific monitor
		name := args[0]
		m, err := database.GetMonitorByName(name)
		if err != nil || m == nil {
			return fmt.Errorf("monitor '%s' not found", name)
		}

		printMonitorStatus(m)
		return nil
	},
}

func printMonitorStatus(m *db.Monitor) {
	status := monitor.CalculateStatus(m)

	fmt.Printf("Monitor: %s\n", color.CyanString(m.Name))
	fmt.Printf("Status:  %s\n", formatStatusLong(status))

	interval := time.Duration(m.IntervalSeconds) * time.Second
	grace := time.Duration(m.GraceSeconds) * time.Second

	fmt.Printf("Interval: %s\n", monitor.FormatDuration(interval))
	if grace > 0 {
		fmt.Printf("Grace:    %s\n", monitor.FormatDuration(grace))
	}

	if m.LastPing != nil {
		fmt.Printf("Last ping: %s (%s)\n",
			m.LastPing.Format("2006-01-02 15:04:05"),
			monitor.TimeAgo(*m.LastPing))

		// Time until late/down
		elapsed := time.Since(*m.LastPing)
		if elapsed < interval {
			remaining := interval - elapsed
			fmt.Printf("Next expected in: %s\n", monitor.FormatDuration(remaining))
		} else if elapsed < interval+grace {
			remaining := interval + grace - elapsed
			fmt.Printf("Grace remaining: %s\n", color.YellowString(monitor.FormatDuration(remaining)))
		} else {
			overdue := elapsed - interval - grace
			fmt.Printf("Overdue by: %s\n", color.RedString(monitor.FormatDuration(overdue)))
		}
	} else {
		fmt.Printf("Last ping: never\n")
	}

	fmt.Printf("Created: %s\n", m.CreatedAt.Format("2006-01-02 15:04:05"))
}

func formatStatusLong(status string) string {
	switch status {
	case "OK":
		return color.GreenString("OK - Running on schedule")
	case "LATE":
		return color.YellowString("LATE - Ping overdue")
	case "DOWN":
		return color.RedString("DOWN - Grace period exceeded")
	default:
		return status
	}
}
