package cassandra

import (
	"context"
	"github.com/gocql/gocql"
	"log"
)

// Session wraps the gocql session.
type Session struct {
	*gocql.Session
}

// New creates a new Cassandra session.
func New(hosts []string, keyspace string) *Session {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("failed to connect to cassandra: %v", err)
	}
	return &Session{session}
}

// InitSchema ensures the projects table exists with the latest columns.
func (s *Session) InitSchema(ctx context.Context) error {
	return s.Query(`CREATE TABLE IF NOT EXISTS projects (
        id text PRIMARY KEY,
        api_key text,
        ttl int,
        search_keys list<text>)`).WithContext(ctx).Exec()
}
