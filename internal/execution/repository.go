package execution

import (
	"context"
	"errors"
)

// Sentinel errors returned by Repository implementations.
var (
	ErrNotFound      = errors.New("execution: not found")
	ErrAlreadyExists = errors.New("execution: already exists")
)

// ListFilter narrows List results. A zero-value field means "don't
// filter on this dimension". Pagination (limit/offset or cursor) is
// deliberately left out until a real caller (dashboard, Milestone 6)
// needs it — adding fields to this struct later is backward compatible.
type ListFilter struct {
	WorkflowID string
	State      State
}

// Repository persists Execution aggregates.
//
// Get/List return clones (see Execution.Clone), and Update replaces
// whatever is stored wholesale — there is no optimistic-concurrency
// check (version/CAS) yet. That's an open question carried into
// Milestone 7's Postgres-backed implementation (see
// docs/architecture.md), not resolved here: adding a version check
// later is backward compatible with this interface since Update
// already takes the full aggregate.
type Repository interface {
	Create(ctx context.Context, e *Execution) error
	Get(ctx context.Context, id string) (*Execution, error)
	Update(ctx context.Context, e *Execution) error
	List(ctx context.Context, filter ListFilter) ([]*Execution, error)
}
