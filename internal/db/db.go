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
	CreatedAt       time.Time
	LastPing        *time.Time
	Status          string // OK, LATE, DOWN
}

type Ping struct {
	ID        int64
	MonitorID int64
	Type      string // success, fail, start
	Timestamp time.Time
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
	return err
}

func (d *DB) CreateMonitor(name string, intervalSeconds, graceSeconds int64) (*Monitor, error) {
	result, err := d.conn.Exec(
		`INSERT INTO monitors (name, interval_seconds, grace_seconds, status) VALUES (?, ?, ?, 'OK')`,
		name, intervalSeconds, graceSeconds,
	)
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
		`SELECT id, name, interval_seconds, grace_seconds, created_at, last_ping, status FROM monitors WHERE id = ?`,
		id,
	)
	return scanMonitor(row)
}

func (d *DB) GetMonitorByName(name string) (*Monitor, error) {
	row := d.conn.QueryRow(
		`SELECT id, name, interval_seconds, grace_seconds, created_at, last_ping, status FROM monitors WHERE name = ?`,
		name,
	)
	return scanMonitor(row)
}

func (d *DB) ListMonitors() ([]*Monitor, error) {
	rows, err := d.conn.Query(
		`SELECT id, name, interval_seconds, grace_seconds, created_at, last_ping, status FROM monitors ORDER BY name`,
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
	now := time.Now()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert ping record
	_, err = tx.Exec(
		`INSERT INTO pings (monitor_id, type, timestamp) VALUES (?, ?, ?)`,
		monitorID, pingType, now,
	)
	if err != nil {
		return err
	}

	// Update monitor's last_ping and status
	if pingType == "success" {
		_, err = tx.Exec(
			`UPDATE monitors SET last_ping = ?, status = 'OK' WHERE id = ?`,
			now, monitorID,
		)
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

func (d *DB) GetPings(monitorID int64, limit int) ([]*Ping, error) {
	rows, err := d.conn.Query(
		`SELECT id, monitor_id, type, timestamp FROM pings WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`,
		monitorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pings []*Ping
	for rows.Next() {
		p := &Ping{}
		if err := rows.Scan(&p.ID, &p.MonitorID, &p.Type, &p.Timestamp); err != nil {
			return nil, err
		}
		pings = append(pings, p)
	}

	return pings, rows.Err()
}

func scanMonitor(row *sql.Row) (*Monitor, error) {
	m := &Monitor{}
	var lastPing sql.NullTime
	err := row.Scan(&m.ID, &m.Name, &m.IntervalSeconds, &m.GraceSeconds, &m.CreatedAt, &lastPing, &m.Status)
	if err != nil {
		return nil, err
	}
	if lastPing.Valid {
		m.LastPing = &lastPing.Time
	}
	return m, nil
}

func scanMonitorRows(rows *sql.Rows) (*Monitor, error) {
	m := &Monitor{}
	var lastPing sql.NullTime
	err := rows.Scan(&m.ID, &m.Name, &m.IntervalSeconds, &m.GraceSeconds, &m.CreatedAt, &lastPing, &m.Status)
	if err != nil {
		return nil, err
	}
	if lastPing.Valid {
		m.LastPing = &lastPing.Time
	}
	return m, nil
}
