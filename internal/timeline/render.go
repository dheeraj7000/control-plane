package timeline

import (
	"fmt"

	"github.com/dheeraj7000/control-plane/internal/events"
)

// render maps one Event to its Entry. Every branch tolerates a missing
// expected Data key — a malformed or partially-populated event
// degrades to a generic-but-still-useful label rather than panicking
// or silently disappearing from the timeline. Observability shouldn't
// have a failure mode where the failure is invisible.
func render(e events.Event) Entry {
	entry := Entry{
		EventID:     e.ID,
		ExecutionID: e.ExecutionID,
		Sequence:    e.Sequence,
		At:          e.OccurredAt,
		Type:        e.Type,
	}

	switch e.Type {
	case events.ExecutionCreated:
		entry.Label, entry.Detail = "Execution", "Created"
	case events.ExecutionStarted:
		entry.Label, entry.Detail = "Execution", "Started"
	case events.ExecutionPaused:
		entry.Label, entry.Detail = "Execution", "Paused"
	case events.ExecutionCompleted:
		entry.Label, entry.Detail = "Execution", "Completed"
	case events.ExecutionFailed:
		entry.Label, entry.Detail = "Execution", withReason("Failed", e)
	case events.ExecutionCancelled:
		entry.Label, entry.Detail = "Execution", "Cancelled"

	case events.StepStarted:
		entry.Label, entry.Detail = stepLabel(e), "Started"
	case events.StepCompleted:
		entry.Label, entry.Detail = stepLabel(e), "Completed"
	case events.StepFailed:
		entry.Label, entry.Detail = stepLabel(e), withReason("Failed", e)

	case events.ToolRequested:
		entry.Label, entry.Detail = toolLabel(e), "Requested"
	case events.ToolExecuted:
		entry.Label, entry.Detail = toolLabel(e), "Completed"
	case events.ToolFailed:
		entry.Label, entry.Detail = toolLabel(e), withReason("Failed", e)

	case events.PolicyEvaluated:
		entry.Label, entry.Detail = "Policy", withReason("Evaluated", e)
	case events.PolicyApproved:
		entry.Label, entry.Detail = "Policy", withReason("Approved", e)
	case events.PolicyDenied:
		entry.Label, entry.Detail = "Policy", withReason("Denied", e)

	case events.BudgetUpdated:
		entry.Label, entry.Detail = "Budget", tokenDelta(e)
	case events.BudgetExceeded:
		entry.Label, entry.Detail = "Budget", "Exceeded"

	case events.RetryScheduled:
		entry.Label, entry.Detail = stepLabel(e), attemptDetail(e)

	default:
		// Unknown to this milestone's renderer (e.g. a newer EventType
		// added without a matching case here). Still show something
		// rather than dropping the event from the timeline.
		entry.Label, entry.Detail = string(e.Type), "—"
	}

	return entry
}

func stringData(e events.Event, key string) (string, bool) {
	v, ok := e.Data[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func stepLabel(e events.Event) string {
	if name, ok := stringData(e, events.DataKeyStepName); ok && name != "" {
		return name
	}
	if id, ok := stringData(e, events.DataKeyStepID); ok && id != "" {
		return id
	}
	return "Step"
}

func toolLabel(e events.Event) string {
	if name, ok := stringData(e, events.DataKeyToolName); ok && name != "" {
		return name
	}
	return "Tool"
}

// withReason appends ": <reason>" to base when the event carries one
// (e.g. a step failure's error message, a policy's name/verdict
// reason), and returns base unchanged otherwise.
func withReason(base string, e events.Event) string {
	if reason, ok := stringData(e, events.DataKeyReason); ok && reason != "" {
		return fmt.Sprintf("%s: %s", base, reason)
	}
	if name, ok := stringData(e, events.DataKeyPolicyName); ok && name != "" {
		return fmt.Sprintf("%s %s", name, base)
	}
	return base
}

// tokenDelta renders events.DataKeyTokenDelta as "+3500 Tokens" (or
// "-3500 Tokens" for a negative delta), matching the spec's example.
// Falls back to a plain "Updated" if the delta isn't present or isn't
// a recognizable numeric type.
func tokenDelta(e events.Event) string {
	v, ok := e.Data[events.DataKeyTokenDelta]
	if !ok {
		return "Updated"
	}
	switch n := v.(type) {
	case int:
		return fmt.Sprintf("%+d Tokens", n)
	case int64:
		return fmt.Sprintf("%+d Tokens", n)
	case float64:
		return fmt.Sprintf("%+.0f Tokens", n)
	default:
		return "Updated"
	}
}

func attemptDetail(e events.Event) string {
	v, ok := e.Data[events.DataKeyAttempt]
	if !ok {
		return "Retry Scheduled"
	}
	switch n := v.(type) {
	case int:
		return fmt.Sprintf("Retry Scheduled (attempt %d)", n)
	case float64:
		return fmt.Sprintf("Retry Scheduled (attempt %.0f)", n)
	default:
		return "Retry Scheduled"
	}
}
