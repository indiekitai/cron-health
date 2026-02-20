package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/indiekitai/cron-health/internal/config"
)

// StatusChange represents a monitor status change event
type StatusChange struct {
	MonitorName string
	OldStatus   string
	NewStatus   string
	Timestamp   time.Time
}

// Notifier handles sending notifications
type Notifier struct {
	cfg *config.Config
}

// New creates a new Notifier
func New(cfg *config.Config) *Notifier {
	return &Notifier{cfg: cfg}
}

// ShouldNotify determines if a notification should be sent for a status change
func (n *Notifier) ShouldNotify(oldStatus, newStatus string) bool {
	for _, event := range n.cfg.NotifyOn {
		switch event {
		case "late":
			if newStatus == "LATE" {
				return true
			}
		case "down":
			if newStatus == "DOWN" {
				return true
			}
		case "recovered":
			if newStatus == "OK" && (oldStatus == "LATE" || oldStatus == "DOWN") {
				return true
			}
		}
	}
	return false
}

// Notify sends notifications through all configured channels
func (n *Notifier) Notify(change StatusChange) {
	if !n.ShouldNotify(change.OldStatus, change.NewStatus) {
		return
	}

	// Send Telegram notification
	if n.cfg.IsTelegramEnabled() {
		go n.sendTelegram(change)
	}

	// Send webhook notification
	webhookURL := n.cfg.GetEffectiveWebhookURL()
	if webhookURL != "" {
		go n.sendWebhook(webhookURL, change)
	}
}

// sendTelegram sends a Telegram notification
func (n *Notifier) sendTelegram(change StatusChange) {
	emoji := getStatusEmoji(change.NewStatus)
	action := getStatusAction(change.OldStatus, change.NewStatus)

	message := fmt.Sprintf("%s *%s* %s\n\nStatus: %s → %s\nTime: %s",
		emoji,
		escapeMarkdown(change.MonitorName),
		action,
		change.OldStatus,
		change.NewStatus,
		change.Timestamp.Format("2006-01-02 15:04:05 UTC"),
	)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage",
		n.cfg.Notifications.Telegram.BotToken)

	params := url.Values{}
	params.Set("chat_id", n.cfg.Notifications.Telegram.ChatID)
	params.Set("text", message)
	params.Set("parse_mode", "Markdown")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(apiURL, params)
	if err != nil {
		log.Printf("Telegram notification failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Telegram notification failed with status: %d", resp.StatusCode)
	}
}

// sendWebhook sends a webhook notification
func (n *Notifier) sendWebhook(webhookURL string, change StatusChange) {
	payload := map[string]string{
		"monitor":    change.MonitorName,
		"old_status": change.OldStatus,
		"new_status": change.NewStatus,
		"timestamp":  change.Timestamp.Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Webhook payload marshal failed: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Webhook request creation failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Webhook notification failed: %v", err)
		return
	}
	resp.Body.Close()
}

func getStatusEmoji(status string) string {
	switch status {
	case "OK":
		return "✅"
	case "LATE":
		return "⚠️"
	case "DOWN":
		return "🔴"
	default:
		return "ℹ️"
	}
}

func getStatusAction(oldStatus, newStatus string) string {
	if newStatus == "OK" && (oldStatus == "LATE" || oldStatus == "DOWN") {
		return "has recovered"
	}
	if newStatus == "LATE" {
		return "is running late"
	}
	if newStatus == "DOWN" {
		return "is down"
	}
	return "status changed"
}

// escapeMarkdown escapes special characters for Telegram Markdown
func escapeMarkdown(s string) string {
	// For Markdown v1, we need to escape: _ * [ ] ( ) ~ ` > # + - = | { } . !
	// But for monitor names, we mainly care about _ and *
	replacer := bytes.NewBuffer(nil)
	for _, c := range s {
		switch c {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!':
			replacer.WriteRune('\\')
		}
		replacer.WriteRune(c)
	}
	return replacer.String()
}
