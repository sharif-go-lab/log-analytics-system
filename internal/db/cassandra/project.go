package cassandra

import (
	"context"
	"github.com/gocql/gocql"
)

// Project represents a single project configuration.
type Project struct {
	ID         string
	APIKey     string
	TTL        int // seconds
	Owner      string
	SearchKeys []string
}

// GetProject retrieves a project by ID.
func (s *Session) GetProject(ctx context.Context, id string) (*Project, error) {
	var p Project
	var keys []string
	if err := s.Query(`SELECT id, api_key, ttl, owner, search_keys FROM projects WHERE id = ? LIMIT 1`, id).
		WithContext(ctx).Consistency(gocql.Quorum).Scan(&p.ID, &p.APIKey, &p.TTL, &p.Owner, &keys); err != nil {
		return nil, err
	}
	p.SearchKeys = keys
	return &p, nil
}

// ListProjects returns all projects.
func (s *Session) ListProjects(ctx context.Context, owner string) ([]Project, error) {
	var query string
	var args []interface{}
	if owner != "" {
		query = `SELECT id, api_key, ttl, owner, search_keys FROM projects WHERE owner = ?`
		args = append(args, owner)
	} else {
		query = `SELECT id, api_key, ttl, owner, search_keys FROM projects`
	}
	iter := s.Query(query, args...).WithContext(ctx).Iter()
	var res []Project
	var p Project
	var keys []string
	for iter.Scan(&p.ID, &p.APIKey, &p.TTL, &p.Owner, &keys) {
		p.SearchKeys = keys
		res = append(res, p)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return res, nil
}

// CreateProject inserts a new project record.
func (s *Session) CreateProject(ctx context.Context, p Project) error {
	return s.Query(`INSERT INTO projects (id, api_key, ttl, owner, search_keys) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.APIKey, p.TTL, p.Owner, p.SearchKeys).
		WithContext(ctx).Exec()
}
