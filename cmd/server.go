package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/config"
	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/server"
)

var (
	serverPort   int
	serverDaemon bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the HTTP server to receive pings",
	Long: `Start the HTTP server that receives pings from your cron jobs.

The server provides these endpoints:
  GET /ping/<name>       - Record a successful ping
  GET /ping/<name>/fail  - Record a failed ping
  GET /ping/<name>/start - Record that a job has started

Examples:
  cron-health server --port 8080
  cron-health server --daemon`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		port := serverPort
		if port == 0 {
			port = cfg.ServerPort
		}
		if port == 0 {
			port = 8080
		}

		srv := server.New(database, cfg, port)
		return srv.Start(serverDaemon)
	},
}

func init() {
	serverCmd.Flags().IntVarP(&serverPort, "port", "p", 0, "Port to listen on (default 8080)")
	serverCmd.Flags().BoolVarP(&serverDaemon, "daemon", "d", false, "Run in daemon mode")
}
