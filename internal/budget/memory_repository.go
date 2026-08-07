package budget

import (
	"context"
	"fmt"
	"sync"
)

type ledgerKey struct {
	scope     Scope
	ownerID   string
	periodKey string
}

// InMemoryRepository is a thread-safe, process-local Repository
// implementation — the real implementation used until a later
// milestone's Postgres-backed one exists, not just a test double, same
// pattern as internal/execution and internal/workflow's repositories.
// Like execution's, it stores and returns Clone()s so calling code
// can't rely on pointer aliasing that a real database wouldn't provide.
type InMemoryRepository struct {
	mu      sync.Mutex
	ledgers map[ledgerKey]*Ledger
}

// NewInMemoryRepository returns an empty repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{ledgers: make(map[ledgerKey]*Ledger)}
}

// GetOrCreate implements Repository.
func (r *InMemoryRepository) GetOrCreate(_ context.Context, scope Scope, ownerID, periodKey string, defaultLimit Limit) (*Ledger, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := ledgerKey{scope, ownerID, periodKey}
	if existing, ok := r.ledgers[key]; ok {
		return existing.Clone(), nil
	}

	l, err := New(scope, ownerID, periodKey, defaultLimit)
	if err != nil {
		return nil, err
	}
	r.ledgers[key] = l
	return l.Clone(), nil
}

// Save implements Repository.
func (r *InMemoryRepository) Save(_ context.Context, l *Ledger) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := ledgerKey{l.Scope(), l.OwnerID(), l.PeriodKey()}
	if _, ok := r.ledgers[key]; !ok {
		return fmt.Errorf("%w: scope=%s owner=%s period=%s", ErrNotFound, l.Scope(), l.OwnerID(), l.PeriodKey())
	}
	r.ledgers[key] = l.Clone()
	return nil
}
