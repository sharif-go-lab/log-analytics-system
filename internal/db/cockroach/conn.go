package cockroach

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// Conn wraps the cockroach db connection.
type Conn struct {
	*sql.DB
}

// New creates a new CockroachDB connection.
func New(dsn string) *Conn {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to cockroachdb: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("cockroach ping failed: %v", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("failed to connect to cockroachdb: %v", err)
	}
	return &Conn{db}
}

// InitSchema creates necessary tables.
func (c *Conn) InitSchema(ctx context.Context) error {
	if _, err := c.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS audit (
        project_id STRING,
        name STRING,
        ts TIMESTAMPTZ
    )`); err != nil {
		return err
	}
	_, err := c.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
        username STRING PRIMARY KEY,
        password STRING
    )`)
	return err
}

// InsertAudit stores a simple audit trail row.
func (c *Conn) InsertAudit(ctx context.Context, projectID, name string, ts time.Time) error {
	_, err := c.ExecContext(ctx, `INSERT INTO audit(project_id, name, ts) VALUES ($1,$2,$3)`, projectID, name, ts)
	return err
}

// CheckUser verifies username/password.
func (c *Conn) CheckUser(ctx context.Context, user, pass string) (bool, error) {
	var hash string
	err := c.QueryRowContext(ctx, `SELECT password FROM users WHERE username=$1`, user).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) != nil {
		return false, nil
	}
	return true, nil
}

// CreateUser registers a new user with hashed password.
func (c *Conn) CreateUser(ctx context.Context, user, pass string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = c.ExecContext(ctx, `INSERT INTO users(username, password) VALUES ($1,$2)`, user, string(hash))
	return err
}

// EnsureUser inserts a user if it does not already exist.
func (c *Conn) EnsureUser(ctx context.Context, user, pass string) error {
	var count int
	if err := c.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE username=$1`, user).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return c.CreateUser(ctx, user, pass)
	}
	return nil
}
