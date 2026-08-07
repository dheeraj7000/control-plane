// Package workflow owns the Workflow aggregate: the immutable template
// (steps, dependencies, metadata) that an Execution is instantiated
// from, plus the Step types (Search, Summarize, Call Tool, Review,
// Wait, Approval, Model Call).
//
// A Workflow is constructed once via New and never mutated afterward —
// New validates the step graph completely (unique IDs, resolvable
// dependencies, no cycles) so that any Workflow value in existence is
// guaranteed structurally sound. There is no separate Validate() step
// callers must remember to invoke.
//
// Definition format: canonical representation is this Go struct,
// serialized as JSON (see the `json` tags on Step and Workflow) for
// API request/response bodies. YAML is a thin convenience layer to add
// later (any YAML-to-JSON converter unmarshals into the same struct)
// rather than a distinct format to design around now.
package workflow

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned (wrapped with details) by New. Callers can
// use errors.Is to distinguish failure kinds without parsing strings.
var (
	ErrEmptyID           = errors.New("workflow: id must not be empty")
	ErrEmptyName         = errors.New("workflow: name must not be empty")
	ErrInvalidVersion    = errors.New("workflow: version must be >= 1")
	ErrNoSteps           = errors.New("workflow: must have at least one step")
	ErrEmptyStepID       = errors.New("workflow: step id must not be empty")
	ErrDuplicateStepID   = errors.New("workflow: duplicate step id")
	ErrInvalidStepType   = errors.New("workflow: invalid step type")
	ErrUnknownDependency = errors.New("workflow: step depends on unknown step id")
	ErrSelfDependency    = errors.New("workflow: step cannot depend on itself")
	ErrCyclicDependency  = errors.New("workflow: step dependency graph has a cycle")
)

// Workflow is the immutable template an Execution is instantiated
// from. Zero value is not useful; construct via New.
type Workflow struct {
	id          string
	name        string
	version     int
	description string
	steps       []Step
	metadata    map[string]string
	createdAt   time.Time
	// topoOrder is computed once in New and cached: since New already
	// proved the graph acyclic to compute it, every existing Workflow
	// value can hand it out without re-validating or erroring.
	topoOrder []string
}

// Option configures optional Workflow fields in New.
type Option func(*Workflow)

// WithDescription sets a human-readable description.
func WithDescription(d string) Option {
	return func(w *Workflow) { w.description = d }
}

// WithMetadata attaches free-form key/value metadata.
func WithMetadata(m map[string]string) Option {
	return func(w *Workflow) { w.metadata = copyStringMap(m) }
}

// New validates and constructs a Workflow. id and name must be
// non-empty, version must be >= 1, steps must be non-empty with unique
// IDs, valid types, and an acyclic dependency graph referencing only
// step IDs present in the same slice.
func New(id, name string, version int, steps []Step, opts ...Option) (Workflow, error) {
	if id == "" {
		return Workflow{}, ErrEmptyID
	}
	if name == "" {
		return Workflow{}, ErrEmptyName
	}
	if version < 1 {
		return Workflow{}, fmt.Errorf("%w: got %d", ErrInvalidVersion, version)
	}
	if len(steps) == 0 {
		return Workflow{}, ErrNoSteps
	}

	stepsCopy := make([]Step, len(steps))
	copy(stepsCopy, steps)

	if err := validateSteps(stepsCopy); err != nil {
		return Workflow{}, err
	}

	order, err := topologicalOrder(stepsCopy)
	if err != nil {
		return Workflow{}, err
	}

	wf := Workflow{
		id:        id,
		name:      name,
		version:   version,
		steps:     stepsCopy,
		createdAt: time.Now().UTC(),
		topoOrder: order,
	}
	for _, opt := range opts {
		opt(&wf)
	}
	return wf, nil
}

func validateSteps(steps []Step) error {
	seen := make(map[string]struct{}, len(steps))
	for _, s := range steps {
		if s.ID == "" {
			return ErrEmptyStepID
		}
		if _, dup := seen[s.ID]; dup {
			return fmt.Errorf("%w: %s", ErrDuplicateStepID, s.ID)
		}
		seen[s.ID] = struct{}{}
		if !s.Type.Valid() {
			return fmt.Errorf("%w: step %s has type %q", ErrInvalidStepType, s.ID, s.Type)
		}
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if dep == s.ID {
				return fmt.Errorf("%w: step %s", ErrSelfDependency, s.ID)
			}
			if _, ok := seen[dep]; !ok {
				return fmt.Errorf("%w: step %s depends on %s", ErrUnknownDependency, s.ID, dep)
			}
		}
	}
	return nil
}

// topologicalOrder runs Kahn's algorithm over the step dependency
// graph. It assumes validateSteps has already run (IDs unique, deps
// resolvable) and only needs to detect cycles.
func topologicalOrder(steps []Step) ([]string, error) {
	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps))
	for _, s := range steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			inDegree[s.ID]++
			dependents[dep] = append(dependents[dep], s.ID)
		}
	}

	var queue []string
	for _, s := range steps { // iterate in input order for deterministic output
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	order := make([]string, 0, len(steps))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, next := range dependents[cur] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(steps) {
		return nil, ErrCyclicDependency
	}
	return order, nil
}

// TopologicalOrder returns step IDs in an order that respects
// dependencies (every step appears after everything in its DependsOn).
// Safe to call on any Workflow value — construction already proved the
// graph acyclic.
func (w Workflow) TopologicalOrder() []string {
	out := make([]string, len(w.topoOrder))
	copy(out, w.topoOrder)
	return out
}

// ID is this workflow's identifier, shared across all its versions.
func (w Workflow) ID() string { return w.id }

// Name is the human-readable workflow name.
func (w Workflow) Name() string { return w.name }

// Version distinguishes revisions of the same ID. Executions pin to a
// specific (ID, Version) pair at creation time — see internal/execution.
func (w Workflow) Version() int { return w.version }

// Description is an optional human-readable summary.
func (w Workflow) Description() string { return w.description }

// CreatedAt is when this version was constructed.
func (w Workflow) CreatedAt() time.Time { return w.createdAt }

// Metadata returns a copy; mutating the result does not affect w.
func (w Workflow) Metadata() map[string]string {
	return copyStringMap(w.metadata)
}

// Steps returns a copy of the step slice. Note this is a shallow copy:
// a Step's DependsOn slice and Config map are still shared with the
// Workflow's internal copy. Treat returned Steps as read-only — this
// milestone doesn't yet defensively deep-copy those nested fields,
// since nothing in the codebase mutates them today; harden if that
// changes.
func (w Workflow) Steps() []Step {
	out := make([]Step, len(w.steps))
	copy(out, w.steps)
	return out
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
