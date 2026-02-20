package monitor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/indiekitai/cron-health/internal/config"
	"github.com/indiekitai/cron-health/internal/db"
	"github.com/indiekitai/cron-health/internal/notify"
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
	// If using cron expression, use next expected time
	if m.NextExpected != nil {
		return calculateStatusFromNextExpected(m)
	}

	// Original interval-based logic
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

// calculateStatusFromNextExpected uses the next_expected field for status calculation
func calculateStatusFromNextExpected(m *db.Monitor) string {
	now := time.Now()
	grace := time.Duration(m.GraceSeconds) * time.Second

	if now.Before(*m.NextExpected) {
		return "OK"
	}

	if now.Before(m.NextExpected.Add(grace)) {
		return "LATE"
	}

	return "DOWN"
}

// UpdateAllStatuses checks and updates all monitor statuses
func UpdateAllStatuses(database *db.DB, cfg *config.Config) error {
	monitors, err := database.ListMonitors()
	if err != nil {
		return err
	}

	notifier := notify.New(cfg)

	for _, m := range monitors {
		newStatus := CalculateStatus(m)
		if newStatus != m.Status {
			oldStatus := m.Status
			if err := database.UpdateMonitorStatus(m.ID, newStatus); err != nil {
				return err
			}

			// Send notification
			notifier.Notify(notify.StatusChange{
				MonitorName: m.Name,
				OldStatus:   oldStatus,
				NewStatus:   newStatus,
				Timestamp:   time.Now(),
			})
		}
	}

	return nil
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

// TimeUntil returns a human-readable "time until" string
func TimeUntil(t time.Time) string {
	remaining := time.Until(t)
	if remaining < 0 {
		return "overdue"
	}

	if remaining < time.Minute {
		return "less than a minute"
	}
	if remaining < time.Hour {
		mins := int(remaining.Minutes())
		if mins == 1 {
			return "in 1 minute"
		}
		return fmt.Sprintf("in %d minutes", mins)
	}
	if remaining < 24*time.Hour {
		hours := int(remaining.Hours())
		mins := int(remaining.Minutes()) % 60
		if hours == 1 && mins == 0 {
			return "in 1 hour"
		}
		if mins == 0 {
			return fmt.Sprintf("in %d hours", hours)
		}
		return fmt.Sprintf("in %dh%dm", hours, mins)
	}

	days := int(remaining.Hours()) / 24
	hours := int(remaining.Hours()) % 24
	if days == 1 && hours == 0 {
		return "in 1 day"
	}
	if hours == 0 {
		return fmt.Sprintf("in %d days", days)
	}
	return fmt.Sprintf("in %dd%dh", days, hours)
}
