package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
)

type projectEnvelope struct {
	Project  domain.LayoutProject                 `json:"project"`
	Requests map[string]application.RequestRecord `json:"requests"`
}

func (s *FileStore) loadEnvelope(id string) (projectEnvelope, error) {
	path, err := s.projectPath(id)
	if err != nil {
		return projectEnvelope{}, err
	}
	var env projectEnvelope
	if err := readJSON(path, &env); err != nil {
		return env, mapNotFound(err)
	}
	if env.Requests == nil {
		env.Requests = map[string]application.RequestRecord{}
	}
	return env, nil
}
func (s *FileStore) LoadProject(ctx context.Context, id string) (*domain.LayoutProject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	env, err := s.loadEnvelope(id)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(env.Project)
	var p domain.LayoutProject
	_ = json.Unmarshal(b, &p)
	return &p, nil
}
func (s *FileStore) FindRequest(ctx context.Context, projectID, requestID string) (*application.RequestRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	env, err := s.loadEnvelope(projectID)
	if err != nil {
		return nil, err
	}
	r, ok := env.Requests[requestID]
	if !ok {
		return nil, application.ErrNotFound
	}
	return &r, nil
}
func (s *FileStore) SaveRequest(ctx context.Context, projectID string, r application.RequestRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.projectPath(projectID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	env, err := s.loadEnvelope(projectID)
	if err != nil {
		return err
	}
	if old, ok := env.Requests[r.RequestID]; ok && old.Fingerprint != r.Fingerprint {
		return application.ErrIdempotencyConflict
	}
	env.Requests[r.RequestID] = r
	return atomicJSON(path, env)
}
func (s *FileStore) ListProjects(ctx context.Context) ([]domain.LayoutProject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matches, err := filepath.Glob(filepath.Join(s.root, "projects", "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]domain.LayoutProject, 0, len(matches))
	for _, path := range matches {
		var env projectEnvelope
		if err := readJSON(path, &env); err != nil {
			return nil, err
		}
		out = append(out, env.Project)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *FileStore) Commit(ctx context.Context, p *domain.LayoutProject, r application.RequestRecord, e application.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.projectPath(p.ProjectID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	env, loadErr := s.loadEnvelope(p.ProjectID)
	if loadErr != nil && !errors.Is(loadErr, application.ErrNotFound) {
		return loadErr
	}
	if errors.Is(loadErr, application.ErrNotFound) {
		env = projectEnvelope{Requests: map[string]application.RequestRecord{}}
	}
	if env.Requests == nil {
		env.Requests = map[string]application.RequestRecord{}
	}
	if old, ok := env.Requests[r.RequestID]; ok && old.Fingerprint != r.Fingerprint {
		return application.ErrIdempotencyConflict
	}
	env.Project = *p
	env.Requests[r.RequestID] = r
	if err := atomicJSON(path, env); err != nil {
		return err
	}
	if err := s.appendEventLocked(e); err != nil {
		return err
	}
	return nil
}
func (s *FileStore) RemoveForTest(id string) error {
	path, err := s.projectPath(id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
