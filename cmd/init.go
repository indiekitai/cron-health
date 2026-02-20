package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/indiekitai/cron-health/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cron-health configuration",
	Long:  `Creates the configuration file at ~/.cron-health/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Exists() {
			path, _ := config.GetConfigPath()
			color.Yellow("Config already exists at %s", path)
			return nil
		}

		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}

		path, _ := config.GetConfigPath()
		color.Green("✓ Created config at %s", path)
		fmt.Println()
		fmt.Println("Edit this file to configure:")
		fmt.Println("  - webhook_url: URL to POST notifications")
		fmt.Println("  - notify_on: [late, down, recovered]")
		fmt.Println("  - server_port: HTTP server port (default 8080)")

		return nil
	},
}
