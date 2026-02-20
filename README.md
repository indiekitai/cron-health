# cron-health

A simple CLI tool for monitoring cron jobs. Open source alternative to [healthchecks.io](https://healthchecks.io).

## Features

- **Single binary** - No dependencies, just one executable
- **SQLite storage** - All data stored locally in `~/.cron-health/data.db`
- **HTTP ping endpoints** - Simple GET requests to record job status
- **Webhook notifications** - Get notified when jobs fail
- **Color-coded output** - Instantly see which jobs are healthy

## Installation

### From source

```bash
go install github.com/indiekitai/cron-health@latest
```

### Build locally

```bash
git clone https://github.com/indiekitai/cron-health.git
cd cron-health
go build -o cron-health .
```

## Quick Start

```bash
# Initialize configuration
cron-health init

# Create a monitor for daily backup (expect ping every 24h, 1h grace period)
cron-health create daily-backup --interval 24h --grace 1h

# Start the HTTP server
cron-health server --port 8080 &

# In your cron job, add a ping at the end
# 0 2 * * * /path/to/backup.sh && curl -s http://localhost:8080/ping/daily-backup

# Check status
cron-health list
cron-health status daily-backup
```

## Commands

### `cron-health init`

Initialize the configuration file at `~/.cron-health/config.yaml`.

```bash
cron-health init
```

### `cron-health create <name>`

Create a new monitor.

```bash
# Create a monitor expecting pings every hour
cron-health create hourly-task --interval 1h

# With grace period (5 minutes late before marking DOWN)
cron-health create daily-backup --interval 24h --grace 1h

# Supported duration formats: 30s, 5m, 1h, 1d, 1h30m
```

### `cron-health list`

List all monitors with their current status.

```bash
cron-health list
```

Output:
```
NAME           STATUS    INTERVAL  LAST PING
daily-backup   ● OK      24h       2 hours ago
hourly-sync    ● LATE    1h        1 hour ago
weekly-report  ● DOWN    7d        10 days ago
```

### `cron-health status [name]`

Show detailed status of a specific monitor or all monitors.

```bash
cron-health status daily-backup
```

Output:
```
Monitor: daily-backup
Status:  OK - Running on schedule
Interval: 24h
Grace:    1h
Last ping: 2024-01-15 02:05:23 (2 hours ago)
Next expected in: 21h55m
Created: 2024-01-01 10:00:00
```

### `cron-health delete <name>`

Delete a monitor and its ping history.

```bash
cron-health delete old-monitor
```

### `cron-health logs <name>`

Show ping history for a monitor.

```bash
cron-health logs daily-backup --limit 10
```

Output:
```
Ping history for 'daily-backup' (last 10):

TIMESTAMP            TYPE
2024-01-15 02:05:23  ✓ success
2024-01-14 02:03:45  ✓ success
2024-01-13 02:04:12  ✓ success
```

### `cron-health server`

Start the HTTP server to receive pings.

```bash
# Start on default port (8080)
cron-health server

# Start on custom port
cron-health server --port 3000

# Run in background (daemon mode)
cron-health server --daemon
```

## HTTP Endpoints

When the server is running, these endpoints are available:

| Endpoint | Description |
|----------|-------------|
| `GET /ping/<name>` | Record a successful ping |
| `GET /ping/<name>/fail` | Record a failed ping |
| `GET /ping/<name>/start` | Record that a job has started (optional) |
| `GET /health` | Health check endpoint |
| `GET /api/monitors` | JSON list of all monitors |

### Usage in cron jobs

```bash
# Simple ping at the end of a job
0 2 * * * /path/to/backup.sh && curl -s http://localhost:8080/ping/daily-backup

# With start/end tracking
0 2 * * * curl -s http://localhost:8080/ping/daily-backup/start && /path/to/backup.sh && curl -s http://localhost:8080/ping/daily-backup

# Report failures
0 2 * * * /path/to/backup.sh && curl -s http://localhost:8080/ping/daily-backup || curl -s http://localhost:8080/ping/daily-backup/fail
```

## Status Transitions

Monitors transition through these states:

1. **OK** (green) - Ping received within expected interval
2. **LATE** (yellow) - Ping is overdue (past interval)
3. **DOWN** (red) - Ping is overdue past the grace period

```
[Last Ping] --> [interval elapsed] --> LATE --> [grace elapsed] --> DOWN
     ^                                                               |
     |___________________________ [ping received] ___________________|
```

## Configuration

Configuration is stored at `~/.cron-health/config.yaml`:

```yaml
# Webhook URL to POST notifications
webhook_url: https://your-webhook-url.com/hook

# When to send notifications: late, down, recovered
notify_on:
  - down
  - recovered

# Default server port
server_port: 8080
```

### Webhook Payload

When a status change occurs, a POST request is sent to the webhook URL:

```json
{
  "monitor": "daily-backup",
  "old_status": "OK",
  "new_status": "DOWN",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Running as a System Service

### systemd

Create `/etc/systemd/system/cron-health.service`:

```ini
[Unit]
Description=cron-health monitoring server
After=network.target

[Service]
Type=simple
User=your-user
ExecStart=/usr/local/bin/cron-health server --port 8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable cron-health
sudo systemctl start cron-health
```

## Data Storage

All data is stored in `~/.cron-health/`:

```
~/.cron-health/
├── config.yaml    # Configuration file
└── data.db        # SQLite database
```

## Examples

### Monitor a backup job

```bash
# Create the monitor
cron-health create nightly-backup --interval 24h --grace 2h

# Add to crontab
# 0 3 * * * /opt/backup/run.sh && curl -s http://localhost:8080/ping/nightly-backup
```

### Monitor multiple services

```bash
cron-health create db-cleanup --interval 1h --grace 10m
cron-health create log-rotate --interval 24h --grace 1h
cron-health create health-check --interval 5m --grace 2m
```

### Integrate with notification services

Configure webhook to send to Slack, Discord, or any HTTP endpoint:

```yaml
webhook_url: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
notify_on:
  - down
  - recovered
```

## License

MIT License
