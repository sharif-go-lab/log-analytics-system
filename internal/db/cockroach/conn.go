package cockroach

import (
	"database/sql"
	"log"
        "context"
        "time"

	_ "github.com/lib/pq"
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
	return &Conn{db}
}

// InsertAudit stores a simple audit trail row.
func (c *Conn) InsertAudit(ctx context.Context, projectID, name string, ts time.Time) error {
    _, err := c.ExecContext(ctx, `INSERT INTO audit(project_id, name, ts) VALUES ($1,$2,$3)`, projectID, name, ts)
    return err
}
