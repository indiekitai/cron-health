package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var listJSON bool

// MonitorJSON represents a monitor in JSON format
type MonitorJSON struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Interval     string `json:"interval"`
	Cron         string `json:"cron"`
	LastPing     string `json:"last_ping"`
	NextExpected string `json:"next_expected"`
}

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

		// Calculate exit code based on worst status
		exitCode := 0
		for _, m := range monitors {
			status := monitor.CalculateStatus(m)
			code := statusToExitCode(status)
			if code > exitCode {
				exitCode = code
			}
		}

		if listJSON {
			return outputListJSON(monitors, exitCode)
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

		if exitCode > 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

func outputListJSON(monitors []*db.Monitor, exitCode int) error {
	var result []MonitorJSON

	for _, m := range monitors {
		status := monitor.CalculateStatus(m)

		var intervalStr string
		if m.IntervalSeconds > 0 {
			interval := time.Duration(m.IntervalSeconds) * time.Second
			intervalStr = monitor.FormatDuration(interval)
		}

		var lastPingStr string
		if m.LastPing != nil {
			lastPingStr = m.LastPing.Format(time.RFC3339)
		}

		var nextExpectedStr string
		if m.NextExpected != nil {
			nextExpectedStr = m.NextExpected.Format(time.RFC3339)
		} else if m.LastPing != nil && m.IntervalSeconds > 0 {
			nextTime := m.LastPing.Add(time.Duration(m.IntervalSeconds) * time.Second)
			nextExpectedStr = nextTime.Format(time.RFC3339)
		}

		result = append(result, MonitorJSON{
			Name:         m.Name,
			Status:       status,
			Interval:     intervalStr,
			Cron:         m.CronExpr,
			LastPing:     lastPingStr,
			NextExpected: nextExpectedStr,
		})
	}

	// Output empty array if no monitors
	if result == nil {
		result = []MonitorJSON{}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(output))

	if exitCode > 0 {
		os.Exit(exitCode)
	}
	return nil
}

// statusToExitCode converts a status to an exit code
// OK=0, LATE=1, DOWN=2
func statusToExitCode(status string) int {
	switch status {
	case "DOWN":
		return 2
	case "LATE":
		return 1
	default:
		return 0
	}
}

func init() {
	listCmd.Flags().BoolVarP(&listJSON, "json", "j", false, "Output in JSON format")
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
