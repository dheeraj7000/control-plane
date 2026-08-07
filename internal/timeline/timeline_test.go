package timeline_test

import (
	"testing"

	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/timeline"
)

func mustEvent(t *testing.T, executionID string, et events.EventType, seq uint64, data map[string]any) events.Event {
	t.Helper()
	e, err := events.New(executionID, et, data)
	if err != nil {
		t.Fatalf("events.New() returned error: %v", err)
	}
	e.Sequence = seq
	return e
}

func TestBuild_OrdersBySequence(t *testing.T) {
	e3 := mustEvent(t, "exec-1", events.ExecutionCompleted, 3, nil)
	e1 := mustEvent(t, "exec-1", events.ExecutionCreated, 1, nil)
	e2 := mustEvent(t, "exec-1", events.ExecutionStarted, 2, nil)

	entries := timeline.Build([]events.Event{e3, e1, e2})
	if len(entries) != 3 {
		t.Fatalf("Build() len = %d, want 3", len(entries))
	}
	wantOrder := []events.EventType{events.ExecutionCreated, events.ExecutionStarted, events.ExecutionCompleted}
	for i, want := range wantOrder {
		if entries[i].Type != want {
			t.Errorf("entries[%d].Type = %s, want %s", i, entries[i].Type, want)
		}
	}
}

func TestBuild_RendersExecutionLifecycle(t *testing.T) {
	tests := []struct {
		et         events.EventType
		wantLabel  string
		wantDetail string
	}{
		{events.ExecutionCreated, "Execution", "Created"},
		{events.ExecutionStarted, "Execution", "Started"},
		{events.ExecutionPaused, "Execution", "Paused"},
		{events.ExecutionCompleted, "Execution", "Completed"},
		{events.ExecutionCancelled, "Execution", "Cancelled"},
	}
	for _, tt := range tests {
		e := mustEvent(t, "exec-1", tt.et, 1, nil)
		entries := timeline.Build([]events.Event{e})
		if entries[0].Label != tt.wantLabel || entries[0].Detail != tt.wantDetail {
			t.Errorf("%s: got Label=%q Detail=%q, want Label=%q Detail=%q",
				tt.et, entries[0].Label, entries[0].Detail, tt.wantLabel, tt.wantDetail)
		}
	}
}

func TestBuild_RendersStepEvents(t *testing.T) {
	e := mustEvent(t, "exec-1", events.StepCompleted, 1, map[string]any{
		events.DataKeyStepName: "Search",
	})
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label != "Search" || entries[0].Detail != "Completed" {
		t.Errorf("got Label=%q Detail=%q, want Search/Completed", entries[0].Label, entries[0].Detail)
	}
}

func TestBuild_RendersToolEvents(t *testing.T) {
	e := mustEvent(t, "exec-1", events.ToolExecuted, 1, map[string]any{
		events.DataKeyToolName: "GitHub",
	})
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label != "GitHub" || entries[0].Detail != "Completed" {
		t.Errorf("got Label=%q Detail=%q, want GitHub/Completed", entries[0].Label, entries[0].Detail)
	}
}

func TestBuild_RendersRemainingEventTypes(t *testing.T) {
	tests := []struct {
		name       string
		e          events.Event
		wantLabel  string
		wantDetail string
	}{
		{
			"ExecutionFailed with reason",
			mustEvent(t, "exec-1", events.ExecutionFailed, 1, map[string]any{events.DataKeyReason: "budget exceeded"}),
			"Execution", "Failed: budget exceeded",
		},
		{
			"StepStarted falls back to step id",
			mustEvent(t, "exec-1", events.StepStarted, 1, map[string]any{events.DataKeyStepID: "fetch"}),
			"fetch", "Started",
		},
		{
			"ToolRequested falls back to generic label",
			mustEvent(t, "exec-1", events.ToolRequested, 1, nil),
			"Tool", "Requested",
		},
		{
			"ToolFailed with reason",
			mustEvent(t, "exec-1", events.ToolFailed, 1, map[string]any{
				events.DataKeyToolName: "GitHub", events.DataKeyReason: "timeout",
			}),
			"GitHub", "Failed: timeout",
		},
		{
			"PolicyEvaluated",
			mustEvent(t, "exec-1", events.PolicyEvaluated, 1, map[string]any{events.DataKeyPolicyName: "Budget Cap"}),
			"Policy", "Budget Cap Evaluated",
		},
		{
			"PolicyApproved",
			mustEvent(t, "exec-1", events.PolicyApproved, 1, map[string]any{events.DataKeyPolicyName: "Budget Cap"}),
			"Policy", "Budget Cap Approved",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := timeline.Build([]events.Event{tt.e})
			if entries[0].Label != tt.wantLabel || entries[0].Detail != tt.wantDetail {
				t.Errorf("got Label=%q Detail=%q, want Label=%q Detail=%q",
					entries[0].Label, entries[0].Detail, tt.wantLabel, tt.wantDetail)
			}
		})
	}
}

func TestBuild_NumericDataTypeVariants(t *testing.T) {
	tests := []struct {
		name       string
		e          events.Event
		wantDetail string
	}{
		{"token delta as int64", mustEvent(t, "exec-1", events.BudgetUpdated, 1, map[string]any{events.DataKeyTokenDelta: int64(-200)}), "-200 Tokens"},
		{"token delta as float64", mustEvent(t, "exec-1", events.BudgetUpdated, 1, map[string]any{events.DataKeyTokenDelta: float64(150)}), "+150 Tokens"},
		{"token delta unrecognized type", mustEvent(t, "exec-1", events.BudgetUpdated, 1, map[string]any{events.DataKeyTokenDelta: "a lot"}), "Updated"},
		{"attempt as float64", mustEvent(t, "exec-1", events.RetryScheduled, 1, map[string]any{events.DataKeyAttempt: float64(3)}), "Retry Scheduled (attempt 3)"},
		{"attempt missing", mustEvent(t, "exec-1", events.RetryScheduled, 1, nil), "Retry Scheduled"},
		{"attempt unrecognized type", mustEvent(t, "exec-1", events.RetryScheduled, 1, map[string]any{events.DataKeyAttempt: "many"}), "Retry Scheduled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := timeline.Build([]events.Event{tt.e})
			if entries[0].Detail != tt.wantDetail {
				t.Errorf("got Detail=%q, want %q", entries[0].Detail, tt.wantDetail)
			}
		})
	}
}

func TestBuild_RendersPolicyDenied(t *testing.T) {
	e := mustEvent(t, "exec-1", events.PolicyDenied, 1, map[string]any{
		events.DataKeyPolicyName: "Filesystem Write",
	})
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label != "Policy" || entries[0].Detail != "Filesystem Write Denied" {
		t.Errorf("got Label=%q Detail=%q, want Policy/'Filesystem Write Denied'", entries[0].Label, entries[0].Detail)
	}
}

func TestBuild_RendersBudgetUpdated(t *testing.T) {
	e := mustEvent(t, "exec-1", events.BudgetUpdated, 1, map[string]any{
		events.DataKeyTokenDelta: 3500,
	})
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label != "Budget" || entries[0].Detail != "+3500 Tokens" {
		t.Errorf("got Label=%q Detail=%q, want Budget/'+3500 Tokens'", entries[0].Label, entries[0].Detail)
	}
}

func TestBuild_RendersBudgetExceeded(t *testing.T) {
	e := mustEvent(t, "exec-1", events.BudgetExceeded, 1, nil)
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label != "Budget" || entries[0].Detail != "Exceeded" {
		t.Errorf("got Label=%q Detail=%q, want Budget/Exceeded", entries[0].Label, entries[0].Detail)
	}
}

func TestBuild_RendersRetryScheduled(t *testing.T) {
	e := mustEvent(t, "exec-1", events.RetryScheduled, 1, map[string]any{
		events.DataKeyStepName: "Search",
		events.DataKeyAttempt:  2,
	})
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label != "Search" || entries[0].Detail != "Retry Scheduled (attempt 2)" {
		t.Errorf("got Label=%q Detail=%q, want Search/'Retry Scheduled (attempt 2)'", entries[0].Label, entries[0].Detail)
	}
}

func TestBuild_MissingDataDegradesGracefully(t *testing.T) {
	// No step name/id at all — must not panic, must still produce a
	// usable (if generic) entry.
	e := mustEvent(t, "exec-1", events.StepFailed, 1, nil)
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label == "" || entries[0].Detail == "" {
		t.Errorf("got empty Label/Detail for event with no Data: %+v", entries[0])
	}
}

func TestBuild_UnknownEventTypeDoesNotPanic(t *testing.T) {
	e, err := events.New("exec-1", events.ExecutionCreated, nil)
	if err != nil {
		t.Fatalf("events.New() returned error: %v", err)
	}
	e.Type = "some.future.event" // simulate a type this renderer predates
	entries := timeline.Build([]events.Event{e})
	if entries[0].Label == "" {
		t.Error("expected a non-empty fallback label for an unknown event type")
	}
}

func TestBuild_ReplayIsDeterministic(t *testing.T) {
	// This is the "replay the execution timeline" success criterion in
	// its read-only sense: Build is pure, so calling it twice on the
	// same events produces identical output.
	evts := []events.Event{
		mustEvent(t, "exec-1", events.ExecutionCreated, 1, nil),
		mustEvent(t, "exec-1", events.ExecutionStarted, 2, nil),
		mustEvent(t, "exec-1", events.ExecutionCompleted, 3, nil),
	}
	first := timeline.Build(evts)
	second := timeline.Build(evts)
	if len(first) != len(second) {
		t.Fatalf("replay produced different lengths: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("entry %d differs between replays: %+v vs %+v", i, first[i], second[i])
		}
	}
}
