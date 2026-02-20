package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/db"
)

var statsDays int

var statsCmd = &cobra.Command{
	Use:   "stats <name>",
	Short: "Show job statistics for a monitor",
	Long: `Show detailed statistics for a monitor including:
- Run counts and success rate
- Duration statistics (average, median, min, max)
- Duration trend (getting faster or slower)
- Recent run history

Example:
  cron-health stats daily-backup
  cron-health stats daily-backup --days 7`,
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
			return fmt.Errorf("monitor '%s' not found", name)
		}

		stats, err := database.GetPingStats(m.ID, statsDays)
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		recentRuns, err := database.GetRecentRuns(m.ID, 5)
		if err != nil {
			return fmt.Errorf("failed to get recent runs: %w", err)
		}

		trend, _ := database.GetDurationTrend(m.ID)

		// Print header
		fmt.Printf("Job Statistics: %s\n", color.CyanString(name))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Run counts
		fmt.Printf("Runs (last %d days): %d\n", statsDays, stats.TotalRuns)
		if stats.TotalRuns > 0 {
			successRate := float64(stats.SuccessCount) / float64(stats.TotalRuns) * 100
			rateStr := fmt.Sprintf("%.1f%%", successRate)
			if successRate == 100 {
				fmt.Printf("Success rate: %s\n", color.GreenString(rateStr))
			} else if successRate >= 90 {
				fmt.Printf("Success rate: %s\n", color.YellowString(rateStr))
			} else {
				fmt.Printf("Success rate: %s\n", color.RedString(rateStr))
			}
		}
		fmt.Println()

		// Duration stats
		if stats.AvgDuration != nil {
			fmt.Println("Duration:")
			fmt.Printf("  Average: %s\n", formatDurationMs(*stats.AvgDuration))
			if len(stats.Durations) > 0 {
				median := calculateMedian(stats.Durations)
				fmt.Printf("  Median:  %s\n", formatDurationMs(median))
			}
			if stats.MinDuration != nil {
				fmt.Printf("  Min:     %s\n", formatDurationMs(*stats.MinDuration))
			}
			if stats.MaxDuration != nil {
				fmt.Printf("  Max:     %s\n", formatDurationMs(*stats.MaxDuration))
			}
			fmt.Println()
		}

		// Trend
		if trend != nil {
			trendStr := ""
			if *trend > 0 {
				trendStr = color.YellowString("+%.0f%% (getting slower)", *trend)
			} else if *trend < 0 {
				trendStr = color.GreenString("%.0f%% (getting faster)", *trend)
			} else {
				trendStr = "stable"
			}
			fmt.Printf("Trend: %s\n", trendStr)
			fmt.Println()
		}

		// Recent runs
		if len(recentRuns) > 0 {
			fmt.Println("Last 5 runs:")
			for _, run := range recentRuns {
				status := color.GreenString("✓")
				if !run.Success {
					status = color.RedString("✗")
				}
				durStr := "failed"
				if run.Success && run.DurationMs != nil {
					durStr = formatDurationMs(*run.DurationMs)
				} else if run.Success {
					durStr = "-"
				}
				fmt.Printf("  %s  %s  %s\n", run.Timestamp.Format("2006-01-02 15:04"), status, durStr)
			}
		}

		return nil
	},
}

func formatDurationMs(ms int64) string {
	d := time.Duration(ms) * time.Millisecond

	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}

	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if d < time.Hour {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}

	hours := int(d.Hours())
	minutes = minutes % 60
	return fmt.Sprintf("%dh %02dm %02ds", hours, minutes, seconds)
}

func calculateMedian(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}

	// Values should already be sorted from the query, but let's make sure
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func init() {
	statsCmd.Flags().IntVarP(&statsDays, "days", "d", 30, "Number of days to analyze")
	rootCmd.AddCommand(statsCmd)
}
