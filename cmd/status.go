package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/monitor"
)

var statusJSON bool
var statusQuiet bool

// StatusJSON represents detailed status in JSON format
type StatusJSON struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	Interval        string `json:"interval,omitempty"`
	Cron            string `json:"cron,omitempty"`
	Grace           string `json:"grace,omitempty"`
	LastPing        string `json:"last_ping,omitempty"`
	NextExpected    string `json:"next_expected,omitempty"`
	CreatedAt       string `json:"created_at"`
	GraceRemaining  string `json:"grace_remaining,omitempty"`
	OverdueBy       string `json:"overdue_by,omitempty"`
}

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
				if !statusQuiet && !statusJSON {
					fmt.Println("No monitors configured.")
				}
				if statusJSON {
					fmt.Println("[]")
				}
				return nil
			}

			// Calculate worst status for exit code and quiet mode
			worstStatus := "OK"
			exitCode := 0
			for _, m := range monitors {
				status := monitor.CalculateStatus(m)
				code := statusToExitCode(status)
				if code > exitCode {
					exitCode = code
					worstStatus = status
				}
			}

			if statusQuiet {
				fmt.Println(worstStatus)
				if exitCode > 0 {
					os.Exit(exitCode)
				}
				return nil
			}

			if statusJSON {
				return outputStatusJSON(monitors, exitCode)
			}

			for i, m := range monitors {
				if i > 0 {
					fmt.Println()
				}
				printMonitorStatus(m)
			}

			if exitCode > 0 {
				os.Exit(exitCode)
			}
			return nil
		}

		// Show specific monitor
		name := args[0]
		m, err := database.GetMonitorByName(name)
		if err != nil || m == nil {
			return fmt.Errorf("monitor '%s' not found", name)
		}

		status := monitor.CalculateStatus(m)
		exitCode := statusToExitCode(status)

		if statusQuiet {
			fmt.Println(status)
			if exitCode > 0 {
				os.Exit(exitCode)
			}
			return nil
		}

		if statusJSON {
			return outputStatusJSON([]*db.Monitor{m}, exitCode)
		}

		printMonitorStatus(m)

		if exitCode > 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

func outputStatusJSON(monitors []*db.Monitor, exitCode int) error {
	var result []StatusJSON

	for _, m := range monitors {
		status := monitor.CalculateStatus(m)
		grace := time.Duration(m.GraceSeconds) * time.Second

		sj := StatusJSON{
			Name:      m.Name,
			Status:    status,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		}

		if m.IntervalSeconds > 0 && m.CronExpr == "" {
			interval := time.Duration(m.IntervalSeconds) * time.Second
			sj.Interval = monitor.FormatDuration(interval)
		}

		if m.CronExpr != "" {
			sj.Cron = m.CronExpr
		}

		if grace > 0 {
			sj.Grace = monitor.FormatDuration(grace)
		}

		if m.LastPing != nil {
			sj.LastPing = m.LastPing.Format(time.RFC3339)
		}

		if m.NextExpected != nil {
			sj.NextExpected = m.NextExpected.Format(time.RFC3339)

			now := time.Now()
			if now.After(*m.NextExpected) && now.Before(m.NextExpected.Add(grace)) {
				remaining := time.Until(m.NextExpected.Add(grace))
				sj.GraceRemaining = monitor.FormatDuration(remaining)
			} else if now.After(m.NextExpected.Add(grace)) {
				overdue := time.Since(m.NextExpected.Add(grace))
				sj.OverdueBy = monitor.FormatDuration(overdue)
			}
		} else if m.LastPing != nil && m.IntervalSeconds > 0 {
			interval := time.Duration(m.IntervalSeconds) * time.Second
			nextTime := m.LastPing.Add(interval)
			sj.NextExpected = nextTime.Format(time.RFC3339)

			elapsed := time.Since(*m.LastPing)
			if elapsed > interval && elapsed < interval+grace {
				remaining := interval + grace - elapsed
				sj.GraceRemaining = monitor.FormatDuration(remaining)
			} else if elapsed > interval+grace {
				overdue := elapsed - interval - grace
				sj.OverdueBy = monitor.FormatDuration(overdue)
			}
		}

		result = append(result, sj)
	}

	// Output single object if only one monitor, array otherwise
	var output []byte
	var err error
	if len(result) == 1 {
		output, err = json.MarshalIndent(result[0], "", "  ")
	} else {
		if result == nil {
			result = []StatusJSON{}
		}
		output, err = json.MarshalIndent(result, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(output))

	if exitCode > 0 {
		os.Exit(exitCode)
	}
	return nil
}

func init() {
	statusCmd.Flags().BoolVarP(&statusJSON, "json", "j", false, "Output in JSON format")
	statusCmd.Flags().BoolVarP(&statusQuiet, "quiet", "q", false, "Output only status (OK, LATE, or DOWN)")
}

func printMonitorStatus(m *db.Monitor) {
	status := monitor.CalculateStatus(m)

	fmt.Printf("Monitor: %s\n", color.CyanString(m.Name))
	fmt.Printf("Status:  %s\n", formatStatusLong(status))

	// Show schedule (cron or interval)
	if m.CronExpr != "" {
		fmt.Printf("Cron:     %s\n", m.CronExpr)
	} else {
		interval := time.Duration(m.IntervalSeconds) * time.Second
		fmt.Printf("Interval: %s\n", monitor.FormatDuration(interval))
	}

	grace := time.Duration(m.GraceSeconds) * time.Second
	if grace > 0 {
		fmt.Printf("Grace:    %s\n", monitor.FormatDuration(grace))
	}

	if m.LastPing != nil {
		fmt.Printf("Last ping: %s (%s)\n",
			m.LastPing.Format("2006-01-02 15:04:05"),
			monitor.TimeAgo(*m.LastPing))
	} else {
		fmt.Printf("Last ping: never\n")
	}

	// Show next expected time
	if m.NextExpected != nil {
		// Cron-based: use stored next_expected
		if time.Now().Before(*m.NextExpected) {
			fmt.Printf("Next expected: %s (%s)\n",
				m.NextExpected.Format("2006-01-02 15:04:05"),
				monitor.TimeUntil(*m.NextExpected))
		} else if time.Now().Before(m.NextExpected.Add(grace)) {
			remaining := time.Until(m.NextExpected.Add(grace))
			fmt.Printf("Grace remaining: %s\n", color.YellowString(monitor.FormatDuration(remaining)))
		} else {
			overdue := time.Since(m.NextExpected.Add(grace))
			fmt.Printf("Overdue by: %s\n", color.RedString(monitor.FormatDuration(overdue)))
		}
	} else if m.LastPing != nil {
		// Interval-based: calculate from last ping
		interval := time.Duration(m.IntervalSeconds) * time.Second
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
