package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/indiekitai/cron-health/internal/config"
)

type DB struct {
	conn *sql.DB
}

type Monitor struct {
	ID              int64
	Name            string
	IntervalSeconds int64
	GraceSeconds    int64
	CronExpr        string     // Cron expression (e.g., "0 2 * * *")
	NextExpected    *time.Time // Next expected ping time (calculated from cron)
	CreatedAt       time.Time
	LastPing        *time.Time
	Status          string // OK, LATE, DOWN
}

type Ping struct {
	ID         int64
	MonitorID  int64
	Type       string // success, fail, start
	Timestamp  time.Time
	DurationMs *int64 // Duration in milliseconds (only for success pings with prior start)
}

func Open() (*DB, error) {
	dbPath, err := config.GetDBPath()
	if err != nil {
		return nil, err
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	// Initial migration
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS monitors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			interval_seconds INTEGER NOT NULL,
			grace_seconds INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_ping DATETIME,
			status TEXT DEFAULT 'OK'
		);

		CREATE TABLE IF NOT EXISTS pings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			monitor_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_pings_monitor_id ON pings(monitor_id);
		CREATE INDEX IF NOT EXISTS idx_pings_timestamp ON pings(timestamp);
	`)
	if err != nil {
		return err
	}

	// Add cron_expr column if not exists
	_, _ = d.conn.Exec(`ALTER TABLE monitors ADD COLUMN cron_expr TEXT DEFAULT ''`)

	// Add next_expected column if not exists
	_, _ = d.conn.Exec(`ALTER TABLE monitors ADD COLUMN next_expected DATETIME`)

	// Add duration_ms column for tracking job duration
	_, _ = d.conn.Exec(`ALTER TABLE pings ADD COLUMN duration_ms INTEGER`)

	return nil
}

func (d *DB) CreateMonitor(name string, intervalSeconds, graceSeconds int64) (*Monitor, error) {
	return d.CreateMonitorWithCron(name, intervalSeconds, graceSeconds, "", nil)
}

func (d *DB) CreateMonitorWithCron(name string, intervalSeconds, graceSeconds int64, cronExpr string, nextExpected *time.Time) (*Monitor, error) {
	var result sql.Result
	var err error

	if nextExpected != nil {
		result, err = d.conn.Exec(
			`INSERT INTO monitors (name, interval_seconds, grace_seconds, cron_expr, next_expected, status) VALUES (?, ?, ?, ?, ?, 'OK')`,
			name, intervalSeconds, graceSeconds, cronExpr, nextExpected,
		)
	} else {
		result, err = d.conn.Exec(
			`INSERT INTO monitors (name, interval_seconds, grace_seconds, cron_expr, status) VALUES (?, ?, ?, ?, 'OK')`,
			name, intervalSeconds, graceSeconds, cronExpr,
		)
	}
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return d.GetMonitor(id)
}

func (d *DB) GetMonitor(id int64) (*Monitor, error) {
	row := d.conn.QueryRow(
		`SELECT id, name, interval_seconds, grace_seconds, COALESCE(cron_expr, ''), next_expected, created_at, last_ping, status FROM monitors WHERE id = ?`,
		id,
	)
	return scanMonitor(row)
}

func (d *DB) GetMonitorByName(name string) (*Monitor, error) {
	row := d.conn.QueryRow(
		`SELECT id, name, interval_seconds, grace_seconds, COALESCE(cron_expr, ''), next_expected, created_at, last_ping, status FROM monitors WHERE name = ?`,
		name,
	)
	return scanMonitor(row)
}

func (d *DB) ListMonitors() ([]*Monitor, error) {
	rows, err := d.conn.Query(
		`SELECT id, name, interval_seconds, grace_seconds, COALESCE(cron_expr, ''), next_expected, created_at, last_ping, status FROM monitors ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*Monitor
	for rows.Next() {
		m, err := scanMonitorRows(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}

	return monitors, rows.Err()
}

func (d *DB) DeleteMonitor(name string) error {
	_, err := d.conn.Exec(`DELETE FROM monitors WHERE name = ?`, name)
	return err
}

func (d *DB) RecordPing(monitorID int64, pingType string) error {
	return d.RecordPingWithNextExpected(monitorID, pingType, nil)
}

func (d *DB) RecordPingWithNextExpected(monitorID int64, pingType string, nextExpected *time.Time) error {
	return d.RecordPingWithDuration(monitorID, pingType, nextExpected, nil)
}

func (d *DB) RecordPingWithDuration(monitorID int64, pingType string, nextExpected *time.Time, durationMs *int64) error {
	now := time.Now()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert ping record with optional duration
	_, err = tx.Exec(
		`INSERT INTO pings (monitor_id, type, timestamp, duration_ms) VALUES (?, ?, ?, ?)`,
		monitorID, pingType, now, durationMs,
	)
	if err != nil {
		return err
	}

	// Update monitor's last_ping, status, and optionally next_expected
	if pingType == "success" {
		if nextExpected != nil {
			_, err = tx.Exec(
				`UPDATE monitors SET last_ping = ?, status = 'OK', next_expected = ? WHERE id = ?`,
				now, nextExpected, monitorID,
			)
		} else {
			_, err = tx.Exec(
				`UPDATE monitors SET last_ping = ?, status = 'OK' WHERE id = ?`,
				now, monitorID,
			)
		}
	} else {
		_, err = tx.Exec(
			`UPDATE monitors SET last_ping = ? WHERE id = ?`,
			now, monitorID,
		)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) UpdateMonitorStatus(id int64, status string) error {
	_, err := d.conn.Exec(`UPDATE monitors SET status = ? WHERE id = ?`, status, id)
	return err
}

func (d *DB) UpdateNextExpected(id int64, nextExpected time.Time) error {
	_, err := d.conn.Exec(`UPDATE monitors SET next_expected = ? WHERE id = ?`, nextExpected, id)
	return err
}

func (d *DB) GetPings(monitorID int64, limit int) ([]*Ping, error) {
	rows, err := d.conn.Query(
		`SELECT id, monitor_id, type, timestamp, duration_ms FROM pings WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`,
		monitorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pings []*Ping
	for rows.Next() {
		p := &Ping{}
		var durationMs sql.NullInt64
		if err := rows.Scan(&p.ID, &p.MonitorID, &p.Type, &p.Timestamp, &durationMs); err != nil {
			return nil, err
		}
		if durationMs.Valid {
			p.DurationMs = &durationMs.Int64
		}
		pings = append(pings, p)
	}

	return pings, rows.Err()
}

// GetLastStartPing returns the most recent start ping for a monitor
func (d *DB) GetLastStartPing(monitorID int64) (*Ping, error) {
	row := d.conn.QueryRow(
		`SELECT id, monitor_id, type, timestamp, duration_ms FROM pings 
		 WHERE monitor_id = ? AND type = 'start' 
		 ORDER BY timestamp DESC LIMIT 1`,
		monitorID,
	)
	p := &Ping{}
	var durationMs sql.NullInt64
	err := row.Scan(&p.ID, &p.MonitorID, &p.Type, &p.Timestamp, &durationMs)
	if err != nil {
		return nil, err
	}
	if durationMs.Valid {
		p.DurationMs = &durationMs.Int64
	}
	return p, nil
}

// PingStats contains statistics for a monitor
type PingStats struct {
	TotalRuns    int
	SuccessCount int
	FailCount    int
	AvgDuration  *int64
	MinDuration  *int64
	MaxDuration  *int64
	Durations    []int64 // For calculating median
}

// GetPingStats returns statistics for a monitor over the last N days
func (d *DB) GetPingStats(monitorID int64, days int) (*PingStats, error) {
	since := time.Now().AddDate(0, 0, -days)

	// Get counts
	var stats PingStats

	// Total runs (success + fail pings, not starts)
	row := d.conn.QueryRow(
		`SELECT COUNT(*) FROM pings 
		 WHERE monitor_id = ? AND type IN ('success', 'fail') AND timestamp >= ?`,
		monitorID, since,
	)
	if err := row.Scan(&stats.TotalRuns); err != nil {
		return nil, err
	}

	// Success count
	row = d.conn.QueryRow(
		`SELECT COUNT(*) FROM pings 
		 WHERE monitor_id = ? AND type = 'success' AND timestamp >= ?`,
		monitorID, since,
	)
	if err := row.Scan(&stats.SuccessCount); err != nil {
		return nil, err
	}

	// Fail count
	row = d.conn.QueryRow(
		`SELECT COUNT(*) FROM pings 
		 WHERE monitor_id = ? AND type = 'fail' AND timestamp >= ?`,
		monitorID, since,
	)
	if err := row.Scan(&stats.FailCount); err != nil {
		return nil, err
	}

	// Duration stats (only for pings with duration_ms set)
	var avgDur, minDur, maxDur sql.NullInt64
	row = d.conn.QueryRow(
		`SELECT AVG(duration_ms), MIN(duration_ms), MAX(duration_ms) FROM pings 
		 WHERE monitor_id = ? AND duration_ms IS NOT NULL AND timestamp >= ?`,
		monitorID, since,
	)
	if err := row.Scan(&avgDur, &minDur, &maxDur); err != nil {
		return nil, err
	}
	if avgDur.Valid {
		stats.AvgDuration = &avgDur.Int64
	}
	if minDur.Valid {
		stats.MinDuration = &minDur.Int64
	}
	if maxDur.Valid {
		stats.MaxDuration = &maxDur.Int64
	}

	// Get all durations for median calculation
	rows, err := d.conn.Query(
		`SELECT duration_ms FROM pings 
		 WHERE monitor_id = ? AND duration_ms IS NOT NULL AND timestamp >= ?
		 ORDER BY duration_ms`,
		monitorID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var dur int64
		if err := rows.Scan(&dur); err != nil {
			return nil, err
		}
		stats.Durations = append(stats.Durations, dur)
	}

	return &stats, rows.Err()
}

// GetRecentRuns returns recent runs with their status and duration for stats display
type RunInfo struct {
	Timestamp  time.Time
	Success    bool
	DurationMs *int64
}

func (d *DB) GetRecentRuns(monitorID int64, limit int) ([]*RunInfo, error) {
	rows, err := d.conn.Query(
		`SELECT timestamp, type, duration_ms FROM pings 
		 WHERE monitor_id = ? AND type IN ('success', 'fail')
		 ORDER BY timestamp DESC LIMIT ?`,
		monitorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*RunInfo
	for rows.Next() {
		r := &RunInfo{}
		var pingType string
		var durationMs sql.NullInt64
		if err := rows.Scan(&r.Timestamp, &pingType, &durationMs); err != nil {
			return nil, err
		}
		r.Success = pingType == "success"
		if durationMs.Valid {
			r.DurationMs = &durationMs.Int64
		}
		runs = append(runs, r)
	}

	return runs, rows.Err()
}

// GetDurationTrend compares recent duration to older duration
// Returns percentage change (positive = slower, negative = faster)
func (d *DB) GetDurationTrend(monitorID int64) (*float64, error) {
	now := time.Now()
	recentStart := now.AddDate(0, 0, -7)
	olderStart := now.AddDate(0, 0, -30)

	// Recent average (last 7 days)
	var recentAvg sql.NullFloat64
	row := d.conn.QueryRow(
		`SELECT AVG(duration_ms) FROM pings 
		 WHERE monitor_id = ? AND duration_ms IS NOT NULL AND timestamp >= ?`,
		monitorID, recentStart,
	)
	if err := row.Scan(&recentAvg); err != nil {
		return nil, err
	}

	// Older average (7-30 days ago)
	var olderAvg sql.NullFloat64
	row = d.conn.QueryRow(
		`SELECT AVG(duration_ms) FROM pings 
		 WHERE monitor_id = ? AND duration_ms IS NOT NULL AND timestamp >= ? AND timestamp < ?`,
		monitorID, olderStart, recentStart,
	)
	if err := row.Scan(&olderAvg); err != nil {
		return nil, err
	}

	if !recentAvg.Valid || !olderAvg.Valid || olderAvg.Float64 == 0 {
		return nil, nil // Not enough data
	}

	trend := ((recentAvg.Float64 - olderAvg.Float64) / olderAvg.Float64) * 100
	return &trend, nil
}

func scanMonitor(row *sql.Row) (*Monitor, error) {
	m := &Monitor{}
	var lastPing sql.NullTime
	var nextExpected sql.NullTime
	err := row.Scan(&m.ID, &m.Name, &m.IntervalSeconds, &m.GraceSeconds, &m.CronExpr, &nextExpected, &m.CreatedAt, &lastPing, &m.Status)
	if err != nil {
		return nil, err
	}
	if lastPing.Valid {
		m.LastPing = &lastPing.Time
	}
	if nextExpected.Valid {
		m.NextExpected = &nextExpected.Time
	}
	return m, nil
}

func scanMonitorRows(rows *sql.Rows) (*Monitor, error) {
	m := &Monitor{}
	var lastPing sql.NullTime
	var nextExpected sql.NullTime
	err := rows.Scan(&m.ID, &m.Name, &m.IntervalSeconds, &m.GraceSeconds, &m.CronExpr, &nextExpected, &m.CreatedAt, &lastPing, &m.Status)
	if err != nil {
		return nil, err
	}
	if lastPing.Valid {
		m.LastPing = &lastPing.Time
	}
	if nextExpected.Valid {
		m.NextExpected = &nextExpected.Time
	}
	return m, nil
}
