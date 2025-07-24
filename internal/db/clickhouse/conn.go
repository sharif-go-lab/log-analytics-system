package clickhouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// Conn wraps the clickhouse connection.
type Conn struct {
	*sql.DB
}

var allowedKey = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// New creates a new ClickHouse connection.
func New(dsn string) *Conn {
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.Fatalf("failed to connect to clickhouse: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("clickhouse ping failed: %v", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("failed to connect to clickhouse: %v", err)
	}
	return &Conn{db}
}

// InitSchema ensures the events table exists.
func (c *Conn) InitSchema(ctx context.Context) error {
	if _, err := c.ExecContext(ctx, `CREATE DATABASE IF NOT EXISTS logs`); err != nil {
		return err
	}
	_, err := c.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS events (
    project_id String,
    name String,
    timestamp Int64,
    expire_at Int64,
    fields String
) ENGINE = MergeTree()
PARTITION BY project_id
ORDER BY (project_id, name, timestamp)
TTL toDateTime(expire_at)`)
	return err
}

// InsertEvent inserts an event record.
func (c *Conn) InsertEvent(ctx context.Context, project, name string, ts int64, expire int64, fields map[string]string) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	_, err = c.ExecContext(ctx, `INSERT INTO events (project_id, name, timestamp, expire_at, fields) VALUES (?, ?, ?, ?, ?)`, project, name, ts, expire, string(data))
	return err
}

// EventCount represents grouped query result.
type EventCount struct {
	Name  string
	Count uint64
	Last  int64
}

// QueryCounts returns counts grouped by event name for a project.
func (c *Conn) QueryCounts(ctx context.Context, project string, filters map[string]string) ([]EventCount, error) {
	now := time.Now().Unix()
	query := `SELECT name, count(), max(timestamp) FROM events WHERE project_id = ? AND expire_at > ?`
	args := []interface{}{project, now}
	for k, v := range filters {
		if !allowedKey.MatchString(k) {
			return nil, errors.New("invalid filter key")
		}
		query += " AND JSONExtractString(fields, '" + k + "') = ?"
		args = append(args, v)
	}
	query += " GROUP BY name"
	rows, err := c.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []EventCount
	for rows.Next() {
		var ec EventCount
		if err := rows.Scan(&ec.Name, &ec.Count, &ec.Last); err != nil {
			return nil, err
		}
		res = append(res, ec)
	}
	return res, nil
}

// Event represents a stored event.
type Event struct {
	Name      string
	Timestamp int64
	Fields    map[string]string
}

// QueryEvents returns events for a project and name with pagination.
func (c *Conn) QueryEvents(ctx context.Context, project, name string, filters map[string]string, limit, offset int) ([]Event, error) {
	now := time.Now().Unix()
	query := `SELECT name, timestamp, fields FROM events WHERE project_id = ? AND expire_at > ?`
	args := []interface{}{project, now}
	if name != "" {
		query += " AND name = ?"
		args = append(args, name)
	}
	for k, v := range filters {
		if !allowedKey.MatchString(k) {
			return nil, errors.New("invalid filter key")
		}
		query += " AND JSONExtractString(fields, '" + k + "') = ?"
		args = append(args, v)
	}
	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := c.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Event
	for rows.Next() {
		var e Event
		var fields string
		if err := rows.Scan(&e.Name, &e.Timestamp, &fields); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(fields), &e.Fields); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	return res, nil
}
