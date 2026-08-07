package workflow

import (
	"context"
	"errors"
)

// Sentinel errors returned by Repository implementations.
var (
	ErrNotFound      = errors.New("workflow: not found")
	ErrAlreadyExists = errors.New("workflow: already exists")
)

// Repository stores immutable Workflow templates, keyed by (ID,
// Version). Because a Workflow is immutable, there is deliberately no
// Update — publishing a change means Create-ing a new version.
//
// The Postgres-backed implementation lands in Milestone 7
// (internal/storage); InMemoryRepository below is what every earlier
// milestone develops and tests against, behind this same interface.
type Repository interface {
	// Create stores a new Workflow. Returns ErrAlreadyExists if
	// (ID, Version) is already taken.
	Create(ctx context.Context, wf Workflow) error
	// Get returns the exact (id, version). Returns ErrNotFound if absent.
	Get(ctx context.Context, id string, version int) (Workflow, error)
	// GetLatest returns the highest Version stored for id. Returns
	// ErrNotFound if no version of id exists.
	GetLatest(ctx context.Context, id string) (Workflow, error)
	// List returns the latest version of every known workflow ID.
	List(ctx context.Context) ([]Workflow, error)
}
