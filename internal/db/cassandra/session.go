package cassandra

import (
	"context"
	"github.com/gocql/gocql"
	"log"
	"time"
)

// Session wraps the gocql session.
type Session struct {
	*gocql.Session
}

// New creates a new Cassandra session.
func New(hosts []string, keyspace string) *Session {
	cluster := gocql.NewCluster(hosts...)
	var session *gocql.Session
	var err error
	for try := 0; try < 30; try++ {
		session, err = cluster.CreateSession()
		if err == nil {
			break
		}
		log.Printf("cassandra connection failed: %v", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("failed to connect to cassandra: %v", err)
	}
	if err := session.Query(`CREATE KEYSPACE IF NOT EXISTS ` + keyspace + ` WITH replication = {
        'class': 'SimpleStrategy', 'replication_factor': 1}`).Exec(); err != nil {
		log.Fatalf("failed to create keyspace: %v", err)
	}
	session.Close()

	cluster.Keyspace = keyspace
	for try := 0; try < 30; try++ {
		session, err = cluster.CreateSession()
		if err == nil {
			break
		}
		log.Printf("cassandra keyspace connection failed: %v", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("failed to connect to cassandra keyspace: %v", err)
	}
	return &Session{session}
}

// InitSchema ensures required tables exist.
func (s *Session) InitSchema(ctx context.Context) error {
	return s.Query(`CREATE TABLE IF NOT EXISTS projects (
        id text PRIMARY KEY,
        api_key text,
        ttl int,
        owner text,
        search_keys set<text>
    )`).WithContext(ctx).Exec()
}
