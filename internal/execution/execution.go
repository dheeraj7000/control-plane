// Package execution owns the Execution aggregate: the runtime instance
// of a Workflow, its state machine, and the per-step progress within
// it. Execution is the primary abstraction of the control plane per
// the spec — but that means "everything belongs to an execution" as a
// *relationship* (budget ledger entries, policy decisions, and events
// all reference an execution ID), not as fields embedded in this
// struct. Embedding them here would make this type grow unboundedly
// and would guess at shapes owned by packages (internal/budget,
// internal/policy, internal/events) that don't exist yet. This
// aggregate owns exactly two things: lifecycle state, and step-run
// progress.
package execution

import (
	"errors"
	"fmt"
	"time"

	"github.com/dheeraj7000/control-plane/internal/workflow"
)

// Sentinel errors, wrapped with details by the methods below.
var (
	ErrEmptyID            = errors.New("execution: id must not be empty")
	ErrNoSteps            = errors.New("execution: workflow has no steps")
	ErrUnknownTargetState = errors.New("execution: unknown target state")
	ErrTerminalState      = errors.New("execution: current state is terminal")
	ErrInvalidTransition  = errors.New("execution: transition not allowed")
	ErrUnknownStep        = errors.New("execution: unknown step id")
	ErrStepTerminal       = errors.New("execution: step is already terminal")
)

// Transition records one state change in an Execution's history.
type Transition struct {
	From   State
	To     State
	At     time.Time
	Reason string
}

// Execution is the runtime instance of a Workflow. Zero value is not
// useful; construct via New.
type Execution struct {
	id              string
	workflowID      string
	workflowVersion int
	agentID         string

	state     State
	createdAt time.Time
	updatedAt time.Time
	history   []Transition

	steps map[string]*StepRun
}

// New creates an Execution pinned to the exact Workflow version passed
// in — later edits to that Workflow ID (a new version) never affect
// executions already running against an earlier one. Every step in wf
// starts StepPending.
func New(id string, wf workflow.Workflow, opts ...Option) (*Execution, error) {
	if id == "" {
		return nil, ErrEmptyID
	}
	steps := wf.Steps()
	if len(steps) == 0 {
		return nil, ErrNoSteps
	}

	stepRuns := make(map[string]*StepRun, len(steps))
	for _, s := range steps {
		stepRuns[s.ID] = &StepRun{StepID: s.ID, Status: StepPending}
	}

	now := time.Now().UTC()
	e := &Execution{
		id:              id,
		workflowID:      wf.ID(),
		workflowVersion: wf.Version(),
		state:           StateCreated,
		createdAt:       now,
		updatedAt:       now,
		steps:           stepRuns,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// Option configures optional Execution fields in New, same pattern as
// workflow.Option.
type Option func(*Execution)

// WithAgentID records which Agent (see internal/agent) started this
// execution. Optional and empty by default — added in Milestone 5 once
// internal/agent existed to reference; earlier executions (and any
// caller that doesn't have an agent context, e.g. a test) simply don't
// set it.
func WithAgentID(agentID string) Option {
	return func(e *Execution) { e.agentID = agentID }
}

// ID is this execution's unique identifier.
func (e *Execution) ID() string { return e.id }

// AgentID is the Agent that started this execution, or "" if none was recorded.
func (e *Execution) AgentID() string { return e.agentID }

// WorkflowID is the Workflow template this execution was instantiated from.
func (e *Execution) WorkflowID() string { return e.workflowID }

// WorkflowVersion is the exact Workflow version this execution is
// pinned to, regardless of later versions published under the same WorkflowID.
func (e *Execution) WorkflowVersion() int { return e.workflowVersion }

// State is the execution's current lifecycle state.
func (e *Execution) State() State { return e.state }

// CreatedAt is when the execution was constructed.
func (e *Execution) CreatedAt() time.Time { return e.createdAt }

// UpdatedAt is when the execution's state or step runs last changed.
func (e *Execution) UpdatedAt() time.Time { return e.updatedAt }

// History returns a copy of every transition this Execution has gone
// through, oldest first.
func (e *Execution) History() []Transition {
	out := make([]Transition, len(e.history))
	copy(out, e.history)
	return out
}

// Transition moves the execution to state `to`, provided the state
// graph (see state.go) allows it from the current state. reason is
// free-form and only used for the history/audit trail (e.g. "budget
// exceeded", "all steps completed", "user cancelled").
func (e *Execution) Transition(to State, reason string) error {
	if !IsValid(to) {
		return fmt.Errorf("%w: %s", ErrUnknownTargetState, to)
	}
	if IsTerminal(e.state) {
		return fmt.Errorf("%w: %s (attempted -> %s)", ErrTerminalState, e.state, to)
	}
	if !CanTransition(e.state, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, e.state, to)
	}

	now := time.Now().UTC()
	e.history = append(e.history, Transition{From: e.state, To: to, At: now, Reason: reason})
	e.state = to
	e.updatedAt = now
	return nil
}

// StepRun returns a copy of the named step's current run state.
func (e *Execution) StepRun(stepID string) (StepRun, bool) {
	sr, ok := e.steps[stepID]
	if !ok {
		return StepRun{}, false
	}
	return sr.clone(), true
}

// StepRuns returns a copy of every step's run state, keyed by step ID.
func (e *Execution) StepRuns() map[string]StepRun {
	out := make(map[string]StepRun, len(e.steps))
	for id, sr := range e.steps {
		out[id] = sr.clone()
	}
	return out
}

// StartStep marks stepID as running and increments its attempt count
// (so a retried step's second call is Attempt 2, not a fresh Attempt 1).
func (e *Execution) StartStep(stepID string) error {
	sr, err := e.mutableStep(stepID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sr.Status = StepRunning
	sr.Attempt++
	sr.StartedAt = &now
	sr.EndedAt = nil
	sr.Error = ""
	return nil
}

// CompleteStep marks stepID as successfully completed.
func (e *Execution) CompleteStep(stepID string) error {
	sr, err := e.mutableStep(stepID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sr.Status = StepCompleted
	sr.EndedAt = &now
	return nil
}

// FailStep marks stepID as failed with the given error message.
func (e *Execution) FailStep(stepID, errMsg string) error {
	sr, err := e.mutableStep(stepID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sr.Status = StepFailed
	sr.EndedAt = &now
	sr.Error = errMsg
	return nil
}

// SkipStep marks stepID as skipped (e.g. a conditional branch not taken).
func (e *Execution) SkipStep(stepID string) error {
	sr, err := e.mutableStep(stepID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sr.Status = StepSkipped
	sr.EndedAt = &now
	return nil
}

// WaitStep marks stepID as waiting (e.g. an Approval step pending a
// human decision, or a Wait step's timer).
func (e *Execution) WaitStep(stepID string) error {
	sr, err := e.mutableStep(stepID)
	if err != nil {
		return err
	}
	sr.Status = StepWaiting
	return nil
}

func (e *Execution) mutableStep(stepID string) (*StepRun, error) {
	sr, ok := e.steps[stepID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownStep, stepID)
	}
	if sr.Status.IsTerminal() {
		return nil, fmt.Errorf("%w: step %s is %s", ErrStepTerminal, stepID, sr.Status)
	}
	return sr, nil
}

// AllStepsCompleted reports whether every step is StepCompleted.
// Skipped steps count as resolved, not as blocking completion.
func (e *Execution) AllStepsCompleted() bool {
	for _, sr := range e.steps {
		if sr.Status != StepCompleted && sr.Status != StepSkipped {
			return false
		}
	}
	return true
}

// AnyStepFailed reports whether at least one step is StepFailed.
func (e *Execution) AnyStepFailed() bool {
	for _, sr := range e.steps {
		if sr.Status == StepFailed {
			return true
		}
	}
	return false
}

// Clone returns a deep copy. Repository implementations use this to
// enforce copy-on-read/copy-on-write semantics — see
// InMemoryRepository — so code written against the in-memory
// repository can't rely on pointer aliasing that won't hold once
// Milestone 7 swaps in the Postgres-backed implementation.
func (e *Execution) Clone() *Execution {
	histCopy := make([]Transition, len(e.history))
	copy(histCopy, e.history)

	stepsCopy := make(map[string]*StepRun, len(e.steps))
	for id, sr := range e.steps {
		clone := sr.clone()
		stepsCopy[id] = &clone
	}

	return &Execution{
		id:              e.id,
		workflowID:      e.workflowID,
		workflowVersion: e.workflowVersion,
		agentID:         e.agentID,
		state:           e.state,
		createdAt:       e.createdAt,
		updatedAt:       e.updatedAt,
		history:         histCopy,
		steps:           stepsCopy,
	}
}
