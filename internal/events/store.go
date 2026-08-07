package events

import "context"

// Store is the durable, replayable event log — the source of truth
// referenced throughout this package's doc comment. Append assigns
// Sequence atomically per ExecutionID, so List always returns a
// gapless, deterministic ordering for one execution's stream.
type Store interface {
	// Append validates e, assigns Sequence (monotonically increasing
	// starting at 1, scoped to e.ExecutionID), and durably records it.
	// Returns the stored copy, Sequence included.
	Append(ctx context.Context, e Event) (Event, error)
	// List returns every event recorded for executionID, ordered by
	// Sequence ascending. Returns an empty slice (not an error) if the
	// execution has no events yet.
	List(ctx context.Context, executionID string) ([]Event, error)
}
