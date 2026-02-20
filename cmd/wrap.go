package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var wrapServer string

var wrapCmd = &cobra.Command{
	Use:   "wrap <name> <command>",
	Short: "Wrap a command with automatic ping tracking",
	Long: `Wrap a command with automatic start/success/fail pings.

This will:
1. Send /ping/<name>/start to the server
2. Execute the command
3. If exit code 0: send /ping/<name> (success)
4. If non-zero exit: send /ping/<name>/fail
5. Pass through stdout/stderr

Use in crontab:
  0 2 * * * cron-health wrap daily-backup "/opt/scripts/backup.sh" --server http://localhost:8080

Examples:
  cron-health wrap backup "./backup.sh"
  cron-health wrap backup "tar czf /backup/data.tar.gz /data" --server http://localhost:8080
  cron-health wrap sync 'rsync -av /src/ /dest/' --server http://myserver:8080`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		command := strings.Join(args[1:], " ")

		// Send start ping
		if err := sendPing(wrapServer, name, "start"); err != nil {
			// Log but don't fail - the job should still run
			fmt.Fprintf(os.Stderr, "Warning: failed to send start ping: %v\n", err)
		}

		// Execute the command
		exitCode := executeCommand(command)

		// Send success or fail ping based on exit code
		if exitCode == 0 {
			if err := sendPing(wrapServer, name, ""); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to send success ping: %v\n", err)
			}
		} else {
			if err := sendPing(wrapServer, name, "fail"); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to send fail ping: %v\n", err)
			}
		}

		// Exit with the same code as the wrapped command
		os.Exit(exitCode)
		return nil
	},
}

func sendPing(server, name, pingType string) error {
	url := fmt.Sprintf("%s/ping/%s", server, name)
	if pingType != "" {
		url = fmt.Sprintf("%s/%s", url, pingType)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func executeCommand(command string) int {
	// Use shell to execute the command
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		// If we can't get the exit code, assume failure
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		return 1
	}

	return 0
}

func init() {
	wrapCmd.Flags().StringVarP(&wrapServer, "server", "s", "http://localhost:8080", "cron-health server URL")
	rootCmd.AddCommand(wrapCmd)
}
