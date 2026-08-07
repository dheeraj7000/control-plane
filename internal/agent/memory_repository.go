package agent

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository is a thread-safe, process-local Repository
// implementation — the real implementation used until a later
// milestone's Postgres-backed one exists, not just a test double, same
// pattern as every other repository in this codebase. Agent is an
// immutable value type (like Workflow), so no Clone-on-read/write
// discipline is needed here — there's no mutable aggregate a caller
// could accidentally alias.
type InMemoryRepository struct {
	mu            sync.RWMutex
	byID          map[string]Agent
	idByTokenHash map[string]string
}

// NewInMemoryRepository returns an empty repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		byID:          make(map[string]Agent),
		idByTokenHash: make(map[string]string),
	}
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, a Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[a.ID()]; exists {
		return fmt.Errorf("%w: id=%s", ErrAlreadyExists, a.ID())
	}
	r.byID[a.ID()] = a
	r.idByTokenHash[a.TokenHash()] = a.ID()
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, id string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.byID[id]
	if !ok {
		return Agent{}, fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}
	return a, nil
}

// FindByTokenHash implements Repository.
func (r *InMemoryRepository) FindByTokenHash(_ context.Context, tokenHash string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.idByTokenHash[tokenHash]
	if !ok {
		return Agent{}, ErrNotFound
	}
	return r.byID[id], nil
}

// List implements Repository.
func (r *InMemoryRepository) List(_ context.Context) ([]Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Agent, 0, len(r.byID))
	for _, a := range r.byID {
		out = append(out, a)
	}
	return out, nil
}
