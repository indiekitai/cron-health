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
)

var logsLimit int
var logsJSON bool

// PingJSON represents a ping record in JSON format
type PingJSON struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
}

var logsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Show ping history for a monitor",
	Long:  `Show the ping history for a specific monitor`,
	Args:  cobra.ExactArgs(1),
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

		pings, err := database.GetPings(m.ID, logsLimit)
		if err != nil {
			return fmt.Errorf("failed to get pings: %w", err)
		}

		if logsJSON {
			return outputLogsJSON(pings)
		}

		if len(pings) == 0 {
			fmt.Printf("No pings recorded for '%s'\n", name)
			return nil
		}

		fmt.Printf("Ping history for '%s' (last %d):\n\n", name, logsLimit)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TIMESTAMP\tTYPE\t")

		for _, p := range pings {
			typeStr := formatPingType(p.Type)
			fmt.Fprintf(w, "%s\t%s\t\n", p.Timestamp.Format("2006-01-02 15:04:05"), typeStr)
		}

		w.Flush()
		return nil
	},
}

func outputLogsJSON(pings []*db.Ping) error {
	var result []PingJSON

	for _, p := range pings {
		result = append(result, PingJSON{
			Timestamp: p.Timestamp.Format(time.RFC3339),
			Type:      p.Type,
		})
	}

	// Output empty array if no pings
	if result == nil {
		result = []PingJSON{}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(output))
	return nil
}

func formatPingType(t string) string {
	switch t {
	case "success":
		return color.GreenString("✓ success")
	case "fail":
		return color.RedString("✗ fail")
	case "start":
		return color.CyanString("▶ start")
	default:
		return t
	}
}

func init() {
	logsCmd.Flags().IntVarP(&logsLimit, "limit", "l", 20, "Number of entries to show")
	logsCmd.Flags().BoolVarP(&logsJSON, "json", "j", false, "Output in JSON format")
}
