// Package timeline projects an Execution's immutable event stream into
// the human-readable Timeline the dashboard renders — the spec's own
// example:
//
//	14:02  Execution Started
//	14:03  Search      Completed
//	14:05  GitHub      Completed
//	14:06  Budget      +3500 Tokens
//	14:07  Policy      Filesystem Write Denied
//	14:09  Execution   Paused
//
// This package is a pure projection: Build takes a slice of
// events.Event and returns a slice of Entry, no I/O, no dependency on
// internal/events.Store or Bus, and no dependency on internal/execution
// or internal/workflow — every label this package renders comes out of
// the event's own Data payload (see events.DataKeyStepName etc.), not
// from reaching into a Workflow to look up a step's name. That keeps
// rendering trivially unit-testable and keeps this package from caring
// how events were produced or delivered.
//
// This is also, deliberately, the full answer to "replay the execution
// timeline" for this milestone: Build is a pure function, so calling it
// again on the same events (fetched via events.Store.List) reproduces
// an identical timeline. That is the safe, read-only sense of "replay"
// carried over as an open question from Milestone 1 — re-executing a
// completed execution's side-effecting steps is a different capability
// entirely and remains explicitly out of scope; see
// docs/architecture.md.
package timeline

import (
	"sort"
	"time"

	"github.com/dheeraj7000/control-plane/internal/events"
)

// Entry is one rendered line in a Timeline. JSON tags added in
// Milestone 6 for the same reason as events.Event's — this is a
// dashboard-facing response body (`GET .../timeline`) and should use
// this project's snake_case convention, not encoding/json's
// no-tags-present fallback to exported Go field names.
type Entry struct {
	EventID     string    `json:"event_id"`
	ExecutionID string    `json:"execution_id"`
	Sequence    uint64    `json:"sequence"`
	At          time.Time `json:"at"`
	// Label is the bold/leading word(s) of the line — "Execution",
	// "Search", "GitHub", "Budget", "Policy" in the example above.
	Label string `json:"label"`
	// Detail is what happened — "Started", "Completed", "+3500
	// Tokens", "Filesystem Write Denied".
	Detail string           `json:"detail"`
	Type   events.EventType `json:"type"`
}

// Build sorts evts by Sequence and renders each into an Entry. Safe to
// call on an unordered or partial slice (e.g. events.Store.List for an
// execution still in progress).
func Build(evts []events.Event) []Entry {
	sorted := Sort(evts)
	out := make([]Entry, len(sorted))
	for i, e := range sorted {
		out[i] = render(e)
	}
	return out
}

// Sort returns a copy of evts ordered by Sequence ascending — the
// definition of "in order" for a single execution's event stream.
func Sort(evts []events.Event) []events.Event {
	out := make([]events.Event, len(evts))
	copy(out, evts)
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out
}
