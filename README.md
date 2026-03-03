[English](README.md) | [中文](README.zh-CN.md)

# cron-health

A simple CLI tool for monitoring cron jobs. Open source alternative to [healthchecks.io](https://healthchecks.io).

## Features

- **Single binary** - No dependencies, just one executable
- **SQLite storage** - All data stored locally in `~/.cron-health/data.db`
- **HTTP ping endpoints** - Simple GET requests to record job status
- **Cron expression support** - Schedule monitors using standard cron syntax
- **Duration tracking** - Automatically track how long jobs take to run
- **Job statistics** - View success rates, duration trends, and run history
- **Wrap command** - Simplify crontab entries with automatic ping tracking
- **Status badges** - SVG badges for embedding in READMEs or dashboards
- **Telegram notifications** - Get notified via Telegram when jobs fail
- **Webhook notifications** - POST to any HTTP endpoint on status changes
- **Interactive TUI** - Terminal UI dashboard for real-time monitoring
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

# Or use cron expression
cron-health create nightly-backup --cron "0 2 * * *" --grace 1h

# Start the HTTP server
cron-health server --port 8080 &

# In your cron job, add a ping at the end
# 0 2 * * * /path/to/backup.sh && curl -s http://localhost:8080/ping/daily-backup

# Check status
cron-health list
cron-health status daily-backup

# Launch interactive dashboard
cron-health tui
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

# Using cron expression (runs at 2am daily)
cron-health create nightly-backup --cron "0 2 * * *" --grace 1h

# Cron expression for every Monday at 9am
cron-health create weekly-report --cron "0 9 * * 1"

# Supported duration formats: 30s, 5m, 1h, 1d, 1h30m
```

### `cron-health list`

List all monitors with their current status.

```bash
cron-health list
```

Output:
```
NAME            STATUS  INTERVAL/CRON  LAST PING      NEXT EXPECTED
daily-backup    ● OK    24h            2 hours ago    in 21h55m
nightly-backup  ● OK    0 2 * * *      8 hours ago    in 15h30m
hourly-sync     ● LATE  1h             1 hour ago     overdue
weekly-report   ● DOWN  0 9 * * 1      10 days ago    overdue
```

#### JSON Output

```bash
cron-health list --json
```

Output:
```json
[
  {
    "name": "daily-backup",
    "status": "OK",
    "interval": "24h",
    "cron": "",
    "last_ping": "2024-01-15T10:30:00Z",
    "next_expected": "2024-01-16T02:00:00Z"
  },
  {
    "name": "nightly-job",
    "status": "LATE",
    "interval": "",
    "cron": "0 2 * * *",
    "last_ping": "2024-01-14T02:00:00Z",
    "next_expected": "2024-01-15T02:00:00Z"
  }
]
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

For cron-based monitors:
```
Monitor: nightly-backup
Status:  OK - Running on schedule
Cron:     0 2 * * *
Grace:    1h
Last ping: 2024-01-15 02:05:23 (8 hours ago)
Next expected: 2024-01-16 02:00:00 (in 15h30m)
Created: 2024-01-01 10:00:00
```

#### Quiet Mode

Output only the status string (useful for scripting):

```bash
# Single monitor
cron-health status daily-backup --quiet
# Output: OK

# All monitors (outputs worst status)
cron-health status --quiet
# Output: LATE
```

#### JSON Output

```bash
cron-health status daily-backup --json
```

Output:
```json
{
  "name": "daily-backup",
  "status": "OK",
  "interval": "24h",
  "grace": "1h",
  "last_ping": "2024-01-15T02:05:23Z",
  "next_expected": "2024-01-16T02:00:00Z",
  "created_at": "2024-01-01T10:00:00Z"
}
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

#### JSON Output

```bash
cron-health logs daily-backup --json
```

Output:
```json
[
  {
    "timestamp": "2024-01-15T02:05:23Z",
    "type": "success",
    "duration_ms": 272000
  },
  {
    "timestamp": "2024-01-14T02:03:45Z",
    "type": "success",
    "duration_ms": 310000
  }
]
```

### `cron-health stats <name>`

Show detailed statistics for a monitor including run counts, success rates, and duration metrics.

```bash
cron-health stats daily-backup
cron-health stats daily-backup --days 7
```

Output:
```
Job Statistics: daily-backup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Runs (last 30 days): 30
Success rate: 96.7%

Duration:
  Average: 5m 32s
  Median:  4m 45s
  Min:     2m 10s
  Max:     12m 05s

Trend: +10% (getting slower)

Last 5 runs:
  2026-02-20 02:00  ✓  4m 32s
  2026-02-19 02:00  ✓  5m 10s
  2026-02-18 02:00  ✗  failed
  2026-02-17 02:00  ✓  4m 55s
  2026-02-16 02:00  ✓  4m 20s
```

Duration tracking requires using `/ping/<name>/start` before your job runs. The duration is automatically calculated when you send the success/fail ping.

### `cron-health wrap <name> <command>`

Wrap a command with automatic ping tracking. This is the easiest way to monitor a cron job.

```bash
cron-health wrap daily-backup "/opt/scripts/backup.sh"
cron-health wrap daily-backup "/opt/scripts/backup.sh" --server http://localhost:8080
```

The wrap command will:
1. Send `/ping/<name>/start` to the server
2. Execute the command
3. If exit code 0: send `/ping/<name>` (success)
4. If non-zero exit: send `/ping/<name>/fail`
5. Pass through stdout/stderr
6. Exit with the same code as the wrapped command

Example in crontab:
```bash
# Old way (manual curl)
0 2 * * * curl -s http://localhost:8080/ping/backup/start && /opt/backup.sh && curl -s http://localhost:8080/ping/backup || curl -s http://localhost:8080/ping/backup/fail

# New way (wrap command)
0 2 * * * cron-health wrap backup "/opt/backup.sh" --server http://localhost:8080
```

The `--server` flag defaults to `http://localhost:8080`.

### `cron-health badge <name>`

Generate an SVG status badge for a monitor.

```bash
# Output badge SVG to file
cron-health badge daily-backup > badge.svg

# View badge SVG
cron-health badge daily-backup | cat
```

The badge shows:
- **Green** - OK (running on schedule)
- **Yellow** - LATE (ping overdue)
- **Red** - DOWN (grace period exceeded)
- **Gray** - Unknown (monitor not found)

### `cron-health tui`

Launch an interactive terminal UI dashboard.

```bash
cron-health tui
```

Keybindings:
- `j/↓` - Move down
- `k/↑` - Move up
- `Enter` - View monitor details
- `a` - Add new monitor
- `d` - Delete monitor
- `r` - Refresh list
- `q/Esc` - Quit

The TUI auto-refreshes every 5 seconds and shows:
- Monitor name
- Current status (with colors)
- Interval or cron expression
- Last ping time
- Next expected ping time

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
| `GET /badge/<name>.svg` | Status badge (SVG image) |

### Badge Endpoint

Embed status badges in your README or dashboard:

```markdown
![Backup Status](http://localhost:8080/badge/daily-backup.svg)
```

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
2. **LATE** (yellow) - Ping is overdue (past interval or next expected time)
3. **DOWN** (red) - Ping is overdue past the grace period

```
[Last Ping] --> [interval elapsed] --> LATE --> [grace elapsed] --> DOWN
     ^                                                               |
     |___________________________ [ping received] ___________________|
```

For cron-based monitors, the "next expected" time is calculated from the cron expression after each successful ping.

## Exit Codes

The `list` and `status` commands use semantic exit codes for scripting:

| Exit Code | Meaning |
|-----------|---------|
| 0 | All monitors OK |
| 1 | At least one monitor LATE |
| 2 | At least one monitor DOWN |

### Examples

```bash
# Check if anything is wrong
cron-health status --quiet || echo "Something is wrong!"

# Script based on exit code
cron-health status --quiet
case $? in
  0) echo "All healthy" ;;
  1) echo "Warning: some jobs late" ;;
  2) echo "Critical: some jobs down" ;;
esac

# Use in CI/CD or monitoring scripts
if ! cron-health status --quiet > /dev/null; then
  send_alert "cron-health detected issues"
fi
```

## Configuration

Configuration is stored at `~/.cron-health/config.yaml`:

```yaml
# Server port
server_port: 8080

# When to send notifications: late, down, recovered
notify_on:
  - late
  - down
  - recovered

# Notification channels
notifications:
  # Telegram notifications
  telegram:
    enabled: true
    bot_token: "123456:ABC-DEF..."
    chat_id: "-1001234567890"

  # Webhook notifications
  webhook:
    enabled: true
    url: "https://your-webhook-url.com/hook"
```

### Telegram Setup

1. Create a bot with [@BotFather](https://t.me/BotFather)
2. Get your bot token
3. Get your chat ID (use [@userinfobot](https://t.me/userinfobot) or check the API)
4. Add the bot to your chat/group
5. Configure in `config.yaml`

Telegram messages include:
- Monitor name
- Status change (OK → LATE → DOWN)
- Timestamp
- Emoji indicators (✅ OK, ⚠️ LATE, 🔴 DOWN)

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
cron-health create nightly-backup --cron "0 3 * * *" --grace 2h

# Add to crontab
# 0 3 * * * /opt/backup/run.sh && curl -s http://localhost:8080/ping/nightly-backup
```

### Monitor multiple services

```bash
cron-health create db-cleanup --interval 1h --grace 10m
cron-health create log-rotate --cron "0 0 * * *" --grace 1h
cron-health create health-check --interval 5m --grace 2m
```

### Integrate with notification services

Configure Telegram and webhook notifications:

```yaml
notify_on:
  - late
  - down
  - recovered

notifications:
  telegram:
    enabled: true
    bot_token: "123456:ABC-DEF..."
    chat_id: "-100123456789"

  webhook:
    enabled: true
    url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

## License

MIT License
