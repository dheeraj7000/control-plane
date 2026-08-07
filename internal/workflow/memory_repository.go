package workflow

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository is a thread-safe, process-local Repository
// implementation. It's the real implementation used by every milestone
// before Milestone 7, not just a test double — internal/app wires it
// in directly until the Postgres-backed one exists.
//
// Workflow values are immutable already, so unlike execution's
// InMemoryRepository this doesn't need defensive cloning to prevent
// aliasing bugs.
type InMemoryRepository struct {
	mu       sync.RWMutex
	versions map[string]map[int]Workflow // id -> version -> workflow
	latest   map[string]int              // id -> highest version stored
}

// NewInMemoryRepository returns an empty repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		versions: make(map[string]map[int]Workflow),
		latest:   make(map[string]int),
	}
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, wf Workflow) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	byVersion, ok := r.versions[wf.ID()]
	if !ok {
		byVersion = make(map[int]Workflow)
		r.versions[wf.ID()] = byVersion
	}
	if _, exists := byVersion[wf.Version()]; exists {
		return fmt.Errorf("%w: id=%s version=%d", ErrAlreadyExists, wf.ID(), wf.Version())
	}
	byVersion[wf.Version()] = wf
	if cur, ok := r.latest[wf.ID()]; !ok || wf.Version() > cur {
		r.latest[wf.ID()] = wf.Version()
	}
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, id string, version int) (Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	byVersion, ok := r.versions[id]
	if !ok {
		return Workflow{}, fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}
	wf, ok := byVersion[version]
	if !ok {
		return Workflow{}, fmt.Errorf("%w: id=%s version=%d", ErrNotFound, id, version)
	}
	return wf, nil
}

// GetLatest implements Repository.
func (r *InMemoryRepository) GetLatest(_ context.Context, id string) (Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	v, ok := r.latest[id]
	if !ok {
		return Workflow{}, fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}
	return r.versions[id][v], nil
}

// List implements Repository.
func (r *InMemoryRepository) List(_ context.Context) ([]Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Workflow, 0, len(r.latest))
	for id, v := range r.latest {
		out = append(out, r.versions[id][v])
	}
	return out, nil
}
