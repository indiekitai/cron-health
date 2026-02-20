package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/indiekitai/cron-health/internal/config"
	"github.com/indiekitai/cron-health/internal/db"
)

// ParseDuration parses human-readable duration strings like "1h", "30m", "1d", "1h30m"
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Handle days specially
	re := regexp.MustCompile(`(\d+)d`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		days, _ := strconv.Atoi(match[:len(match)-1])
		return fmt.Sprintf("%dh", days*24)
	})

	return time.ParseDuration(s)
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, mins)
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

// CalculateStatus determines the current status of a monitor
func CalculateStatus(m *db.Monitor) string {
	if m.LastPing == nil {
		// Never pinged - check against creation time
		elapsed := time.Since(m.CreatedAt)
		interval := time.Duration(m.IntervalSeconds) * time.Second
		grace := time.Duration(m.GraceSeconds) * time.Second

		if elapsed > interval+grace {
			return "DOWN"
		}
		if elapsed > interval {
			return "LATE"
		}
		return "OK"
	}

	elapsed := time.Since(*m.LastPing)
	interval := time.Duration(m.IntervalSeconds) * time.Second
	grace := time.Duration(m.GraceSeconds) * time.Second

	if elapsed > interval+grace {
		return "DOWN"
	}
	if elapsed > interval {
		return "LATE"
	}
	return "OK"
}

// UpdateAllStatuses checks and updates all monitor statuses
func UpdateAllStatuses(database *db.DB, cfg *config.Config) error {
	monitors, err := database.ListMonitors()
	if err != nil {
		return err
	}

	for _, m := range monitors {
		newStatus := CalculateStatus(m)
		if newStatus != m.Status {
			oldStatus := m.Status
			if err := database.UpdateMonitorStatus(m.ID, newStatus); err != nil {
				return err
			}

			// Send notification if configured
			if cfg.WebhookURL != "" {
				shouldNotify := false
				for _, n := range cfg.NotifyOn {
					if (n == "late" && newStatus == "LATE") ||
						(n == "down" && newStatus == "DOWN") ||
						(n == "recovered" && newStatus == "OK" && (oldStatus == "LATE" || oldStatus == "DOWN")) {
						shouldNotify = true
						break
					}
				}
				if shouldNotify {
					go sendWebhook(cfg.WebhookURL, m.Name, oldStatus, newStatus)
				}
			}
		}
	}

	return nil
}

func sendWebhook(url, monitorName, oldStatus, newStatus string) {
	payload := map[string]string{
		"monitor":    monitorName,
		"old_status": oldStatus,
		"new_status": newStatus,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// TimeAgo returns a human-readable "time ago" string
func TimeAgo(t time.Time) string {
	elapsed := time.Since(t)

	if elapsed < time.Minute {
		return "just now"
	}
	if elapsed < time.Hour {
		mins := int(elapsed.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if elapsed < 24*time.Hour {
		hours := int(elapsed.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	days := int(elapsed.Hours()) / 24
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
