package cassandra

import (
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
