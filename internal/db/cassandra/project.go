package cassandra

import (
    "context"
    "github.com/gocql/gocql"
)

// Project represents a single project configuration.
type Project struct {
    ID     string
    APIKey string
    TTL    int // seconds
}

// GetProject retrieves a project by ID.
func (s *Session) GetProject(ctx context.Context, id string) (*Project, error) {
    var p Project
    if err := s.Query(`SELECT id, api_key, ttl FROM projects WHERE id = ? LIMIT 1`, id).WithContext(ctx).Consistency(gocql.Quorum).Scan(&p.ID, &p.APIKey, &p.TTL); err != nil {
        return nil, err
    }
    return &p, nil
}
