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
		fmt.Fprintln(w, "NAME\tSTATUS\tINTERVAL/CRON\tLAST PING\tNEXT EXPECTED\t")

		for _, m := range monitors {
			status := monitor.CalculateStatus(m)
			statusStr := formatStatus(status)

			var scheduleStr string
			if m.CronExpr != "" {
				scheduleStr = m.CronExpr
			} else {
				interval := time.Duration(m.IntervalSeconds) * time.Second
				scheduleStr = monitor.FormatDuration(interval)
			}

			var lastPingStr string
			if m.LastPing != nil {
				lastPingStr = monitor.TimeAgo(*m.LastPing)
			} else {
				lastPingStr = "never"
			}

			var nextExpectedStr string
			if m.NextExpected != nil {
				nextExpectedStr = monitor.TimeUntil(*m.NextExpected)
			} else if m.LastPing != nil {
				nextTime := m.LastPing.Add(time.Duration(m.IntervalSeconds) * time.Second)
				nextExpectedStr = monitor.TimeUntil(nextTime)
			} else {
				nextExpectedStr = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", m.Name, statusStr, scheduleStr, lastPingStr, nextExpectedStr)
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
