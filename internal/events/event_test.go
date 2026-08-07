package events_test

import (
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/events"
)

func TestNew_Valid(t *testing.T) {
	e, err := events.New("exec-1", events.ExecutionStarted, map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if e.ExecutionID != "exec-1" || e.Type != events.ExecutionStarted {
		t.Errorf("unexpected event: %+v", e)
	}
	if e.ID == "" {
		t.Error("ID should be assigned")
	}
	if e.OccurredAt.IsZero() {
		t.Error("OccurredAt should be assigned")
	}
	if e.Sequence != 0 {
		t.Errorf("Sequence = %d, want 0 (unassigned until Store.Append)", e.Sequence)
	}
}

func TestNew_EmptyExecutionID(t *testing.T) {
	if _, err := events.New("", events.ExecutionStarted, nil); !errors.Is(err, events.ErrEmptyExecutionID) {
		t.Fatalf("New() error = %v, want ErrEmptyExecutionID", err)
	}
}

func TestNew_InvalidType(t *testing.T) {
	if _, err := events.New("exec-1", "not-a-real-type", nil); !errors.Is(err, events.ErrInvalidEventType) {
		t.Fatalf("New() error = %v, want ErrInvalidEventType", err)
	}
}

func TestEventType_Valid(t *testing.T) {
	known := []events.EventType{
		events.ExecutionCreated, events.ExecutionStarted, events.ExecutionPaused,
		events.ExecutionCompleted, events.ExecutionFailed, events.ExecutionCancelled,
		events.StepStarted, events.StepCompleted, events.StepFailed,
		events.ToolRequested, events.ToolExecuted, events.ToolFailed,
		events.PolicyEvaluated, events.PolicyApproved, events.PolicyDenied,
		events.BudgetUpdated, events.BudgetExceeded,
		events.RetryScheduled,
	}
	for _, et := range known {
		if !et.Valid() {
			t.Errorf("Valid(%s) = false, want true", et)
		}
	}
	if events.EventType("bogus").Valid() {
		t.Error(`Valid("bogus") = true, want false`)
	}
}
