// Package events defines the Event envelope and the Store/Bus
// interfaces every subsystem uses to announce and observe state
// changes (ExecutionStarted, StepCompleted, PolicyDenied, ...) — see
// design principle #3: no subsystem should directly manipulate another
// when an event-driven approach fits.
//
// Store vs. Bus is the Milestone 1 architecture decision made explicit
// as code: Store is the durable, replayable log (Postgres in
// Milestone 7, in-memory today) and is the source of truth. Bus is
// ephemeral pub/sub fan-out for live delivery (NATS once something
// needs cross-instance delivery — e.g. the WebSocket streaming
// endpoints in a later milestone; in-memory today) and may drop events
// under backpressure. Anything that must not be missed reads from
// Store; Bus is for "update the UI live if you're watching right now".
//
// Every event belongs to exactly one Execution (ExecutionID is
// required) — control-plane-level events that aren't scoped to a
// single execution (e.g. "a tool was registered") are internal/audit's
// concern, not this package's.
package events

import (
	"errors"
	"fmt"
	"time"

	"github.com/dheeraj7000/control-plane/pkg/id"
)

// EventType is the closed set of events this control plane emits,
// taken directly from the spec's named event chain and its
// "PolicyDenied / BudgetExceeded / RetryScheduled" examples.
type EventType string

// The eighteen event types this milestone's producers and renderer
// (internal/timeline) agree on, taken from the spec's named event
// chain and examples.
const (
	ExecutionCreated   EventType = "execution.created"
	ExecutionStarted   EventType = "execution.started"
	ExecutionPaused    EventType = "execution.paused"
	ExecutionCompleted EventType = "execution.completed"
	ExecutionFailed    EventType = "execution.failed"
	ExecutionCancelled EventType = "execution.cancelled"

	StepStarted   EventType = "step.started"
	StepCompleted EventType = "step.completed"
	StepFailed    EventType = "step.failed"

	ToolRequested EventType = "tool.requested"
	ToolExecuted  EventType = "tool.executed"
	ToolFailed    EventType = "tool.failed"

	PolicyEvaluated EventType = "policy.evaluated"
	PolicyApproved  EventType = "policy.approved"
	PolicyDenied    EventType = "policy.denied"

	BudgetUpdated  EventType = "budget.updated"
	BudgetExceeded EventType = "budget.exceeded"

	RetryScheduled EventType = "retry.scheduled"
)

// Valid reports whether t is one of the known event types.
func (t EventType) Valid() bool {
	switch t {
	case ExecutionCreated, ExecutionStarted, ExecutionPaused, ExecutionCompleted, ExecutionFailed, ExecutionCancelled,
		StepStarted, StepCompleted, StepFailed,
		ToolRequested, ToolExecuted, ToolFailed,
		PolicyEvaluated, PolicyApproved, PolicyDenied,
		BudgetUpdated, BudgetExceeded,
		RetryScheduled:
		return true
	default:
		return false
	}
}

// Well-known Data keys. Producers and internal/timeline's renderer
// agree on these names instead of each inventing their own strings —
// event payloads stay untyped (map[string]any, same rationale as
// workflow.Step.Config: the producers that would justify concrete
// per-type structs — Policy, Budget, Adapters — don't exist yet) but
// at least the common keys are discoverable and typo-proof.
const (
	DataKeyStepID     = "step_id"
	DataKeyStepName   = "step_name"
	DataKeyToolName   = "tool_name"
	DataKeyPolicyName = "policy_name"
	DataKeyReason     = "reason"
	DataKeyTokenDelta = "token_delta"
	DataKeyAttempt    = "attempt"
)

// Sentinel errors returned by New and by Store/Bus implementations.
var (
	ErrEmptyExecutionID = errors.New("events: execution id must not be empty")
	ErrInvalidEventType = errors.New("events: invalid event type")
)

// Event is one immutable fact about an Execution. Sequence is left
// zero until a Store assigns it on Append — see store.go.
type Event struct {
	ID          string
	ExecutionID string
	Type        EventType
	OccurredAt  time.Time
	Sequence    uint64
	Data        map[string]any
}

// New constructs an Event, validating executionID and eventType and
// assigning ID/OccurredAt. Sequence is left unset — only a Store
// assigns that, atomically, at Append time.
func New(executionID string, eventType EventType, data map[string]any) (Event, error) {
	if executionID == "" {
		return Event{}, ErrEmptyExecutionID
	}
	if !eventType.Valid() {
		return Event{}, fmt.Errorf("%w: %s", ErrInvalidEventType, eventType)
	}
	return Event{
		ID:          id.New("evt"),
		ExecutionID: executionID,
		Type:        eventType,
		OccurredAt:  time.Now().UTC(),
		Data:        copyAnyMap(data),
	}, nil
}

// clone returns a copy of e. Like Workflow.Steps() in Milestone 2,
// this is a shallow copy of Data's values — treat retrieved Events as
// read-only.
func (e Event) clone() Event {
	out := e
	out.Data = copyAnyMap(e.Data)
	return out
}

func copyAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
