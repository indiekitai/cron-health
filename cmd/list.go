package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all monitors",
	Long:  `List all monitors with their current status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		monitors, err := database.ListMonitors()
		if err != nil {
			return fmt.Errorf("failed to list monitors: %w", err)
		}

		if len(monitors) == 0 {
			fmt.Println("No monitors configured.")
			fmt.Println("Create one with: cron-health create <name> --interval <duration>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tINTERVAL\tLAST PING\t")

		for _, m := range monitors {
			status := monitor.CalculateStatus(m)
			statusStr := formatStatus(status)

			interval := time.Duration(m.IntervalSeconds) * time.Second
			intervalStr := monitor.FormatDuration(interval)

			var lastPingStr string
			if m.LastPing != nil {
				lastPingStr = monitor.TimeAgo(*m.LastPing)
			} else {
				lastPingStr = "never"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", m.Name, statusStr, intervalStr, lastPingStr)
		}

		w.Flush()
		return nil
	},
}

func formatStatus(status string) string {
	switch status {
	case "OK":
		return color.GreenString("● OK")
	case "LATE":
		return color.YellowString("● LATE")
	case "DOWN":
		return color.RedString("● DOWN")
	default:
		return status
	}
}
