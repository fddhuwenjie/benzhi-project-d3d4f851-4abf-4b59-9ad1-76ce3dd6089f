package application

import (
	"context"

	"wayfinding-release-gate/internal/domain"
)

func (s *Service) projectView(ctx context.Context, id string) (*domain.LayoutProject, error) {
	s.viewMu.RLock()
	cached := s.viewCache[id]
	s.viewMu.RUnlock()
	if cached != nil {
		return cloneProject(cached), nil
	}

	project, err := s.repo.LoadProject(ctx, id)
	if err != nil {
		return nil, err
	}
	s.viewMu.Lock()
	s.viewCache[id] = cloneProject(project)
	s.viewMu.Unlock()
	return project, nil
}

func (s *Service) invalidateProjectView(id string) {
	s.viewMu.Lock()
	delete(s.viewCache, id)
	s.viewMu.Unlock()
}
