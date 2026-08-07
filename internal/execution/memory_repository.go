package execution

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository is a thread-safe, process-local Repository
// implementation. It's the real implementation every milestone before
// Milestone 7 runs against, not just a test double.
//
// Every stored/returned Execution is a Clone(): callers cannot mutate
// an in-flight Execution and expect it persisted without an explicit
// Update call, matching how the real Postgres-backed implementation
// will behave. This is deliberate — it would be easy (and misleading)
// to make the in-memory version "just work" via shared pointers, but
// that would hide bugs that only surface once Milestone 7 lands.
type InMemoryRepository struct {
	mu   sync.RWMutex
	data map[string]*Execution
}

// NewInMemoryRepository returns an empty repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{data: make(map[string]*Execution)}
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, e *Execution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data[e.ID()]; exists {
		return fmt.Errorf("%w: id=%s", ErrAlreadyExists, e.ID())
	}
	r.data[e.ID()] = e.Clone()
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, id string) (*Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}
	return e.Clone(), nil
}

// Update implements Repository.
func (r *InMemoryRepository) Update(_ context.Context, e *Execution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[e.ID()]; !ok {
		return fmt.Errorf("%w: id=%s", ErrNotFound, e.ID())
	}
	r.data[e.ID()] = e.Clone()
	return nil
}

// List implements Repository.
func (r *InMemoryRepository) List(_ context.Context, filter ListFilter) ([]*Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Execution, 0, len(r.data))
	for _, e := range r.data {
		if filter.WorkflowID != "" && e.WorkflowID() != filter.WorkflowID {
			continue
		}
		if filter.State != "" && e.State() != filter.State {
			continue
		}
		out = append(out, e.Clone())
	}
	return out, nil
}
