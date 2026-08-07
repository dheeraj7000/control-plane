package events

import (
	"context"
	"fmt"
)

// Recorder is what producers (a future Execution Manager, Policy
// Engine, Budget Engine, ...) depend on instead of wiring a Store and
// a Bus separately. Record durably appends first, then publishes the
// exact stored copy (Sequence included) — so a live subscriber and a
// later replay via Store.List see identical Sequence numbers for the
// same event.
type Recorder struct {
	store Store
	bus   Bus
}

// NewRecorder builds a Recorder over the given Store and Bus.
func NewRecorder(store Store, bus Bus) *Recorder {
	return &Recorder{store: store, bus: bus}
}

// Record constructs, durably stores, and publishes an event. If
// publishing fails after a successful store, the event is NOT lost —
// it's durably recorded and will appear on replay — but the error is
// still surfaced so the caller (which may have a logger this package
// doesn't) can decide whether that's worth noting.
func (r *Recorder) Record(ctx context.Context, executionID string, eventType EventType, data map[string]any) (Event, error) {
	e, err := New(executionID, eventType, data)
	if err != nil {
		return Event{}, err
	}

	stored, err := r.store.Append(ctx, e)
	if err != nil {
		return Event{}, fmt.Errorf("events: append: %w", err)
	}

	if err := r.bus.Publish(ctx, stored); err != nil {
		return stored, fmt.Errorf("events: stored (sequence=%d) but failed to publish: %w", stored.Sequence, err)
	}
	return stored, nil
}

// Store exposes the underlying durable log for reads (List, for
// replay/timeline projection) — Record is the only write path, but
// callers legitimately need read access to the same Store without
// this package growing a parallel Get/List surface on Recorder itself.
func (r *Recorder) Store() Store { return r.store }

// Bus exposes the underlying live fan-out for Subscribe — same
// rationale as Store above.
func (r *Recorder) Bus() Bus { return r.bus }
