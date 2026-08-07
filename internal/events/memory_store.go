package events

import (
	"context"
	"sync"
)

// InMemoryStore is a thread-safe, process-local Store implementation.
// Like the Milestone 2 repositories, this is the real implementation
// every milestone runs on before Milestone 7 swaps in a Postgres-backed
// one — not just a test double.
type InMemoryStore struct {
	mu      sync.Mutex
	byExec  map[string][]Event
	nextSeq map[string]uint64
}

// NewInMemoryStore returns an empty store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		byExec:  make(map[string][]Event),
		nextSeq: make(map[string]uint64),
	}
}

// Append implements Store.
func (s *InMemoryStore) Append(_ context.Context, e Event) (Event, error) {
	if e.ExecutionID == "" {
		return Event{}, ErrEmptyExecutionID
	}
	if !e.Type.Valid() {
		return Event{}, ErrInvalidEventType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextSeq[e.ExecutionID]++
	e.Sequence = s.nextSeq[e.ExecutionID]
	stored := e.clone()
	s.byExec[e.ExecutionID] = append(s.byExec[e.ExecutionID], stored)
	return stored, nil
}

// List implements Store.
func (s *InMemoryStore) List(_ context.Context, executionID string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	src := s.byExec[executionID]
	out := make([]Event, len(src))
	for i, e := range src {
		out[i] = e.clone()
	}
	return out, nil
}
