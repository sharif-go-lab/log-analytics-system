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

// ListProjects returns all projects.
func (s *Session) ListProjects(ctx context.Context) ([]Project, error) {
    iter := s.Query(`SELECT id, api_key, ttl FROM projects`).WithContext(ctx).Iter()
    var res []Project
    var p Project
    for iter.Scan(&p.ID, &p.APIKey, &p.TTL) {
        res = append(res, p)
    }
    if err := iter.Close(); err != nil {
        return nil, err
    }
    return res, nil
}

// CreateProject inserts a new project record.
func (s *Session) CreateProject(ctx context.Context, p Project) error {
    return s.Query(`INSERT INTO projects (id, api_key, ttl) VALUES (?, ?, ?)`, p.ID, p.APIKey, p.TTL).
        WithContext(ctx).Exec()
}
