package execution_test

import (
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

func testWorkflow(t *testing.T) workflow.Workflow {
	t.Helper()
	wf, err := workflow.New("wf-1", "Research", 1, []workflow.Step{
		{ID: "fetch", Type: workflow.StepTypeSearch},
		{ID: "summarize", Type: workflow.StepTypeSummarize, DependsOn: []string{"fetch"}},
	})
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}
	return wf
}

func TestNew_InitializesStepsPending(t *testing.T) {
	wf := testWorkflow(t)
	e, err := execution.New("exec-1", wf)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if e.State() != execution.StateCreated {
		t.Errorf("State() = %s, want %s", e.State(), execution.StateCreated)
	}
	if e.WorkflowID() != "wf-1" || e.WorkflowVersion() != 1 {
		t.Errorf("workflow ref = %s/%d, want wf-1/1", e.WorkflowID(), e.WorkflowVersion())
	}

	runs := e.StepRuns()
	if len(runs) != 2 {
		t.Fatalf("StepRuns() len = %d, want 2", len(runs))
	}
	for id, sr := range runs {
		if sr.Status != execution.StepPending {
			t.Errorf("step %s status = %s, want pending", id, sr.Status)
		}
	}
}

func TestNew_EmptyID(t *testing.T) {
	wf := testWorkflow(t)
	if _, err := execution.New("", wf); !errors.Is(err, execution.ErrEmptyID) {
		t.Fatalf("New() error = %v, want ErrEmptyID", err)
	}
}

func TestTransition_ValidPath(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	path := []execution.State{execution.StateQueued, execution.StateRunning, execution.StateCompleted}
	for _, s := range path {
		if err := e.Transition(s, "test"); err != nil {
			t.Fatalf("Transition(%s) returned error: %v", s, err)
		}
	}
	if e.State() != execution.StateCompleted {
		t.Errorf("final State() = %s, want %s", e.State(), execution.StateCompleted)
	}
}

func TestTransition_InvalidRejected(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	err = e.Transition(execution.StateCompleted, "skip ahead")
	if !errors.Is(err, execution.ErrInvalidTransition) {
		t.Fatalf("Transition() error = %v, want ErrInvalidTransition", err)
	}
	if e.State() != execution.StateCreated {
		t.Errorf("State() = %s after rejected transition, want unchanged %s", e.State(), execution.StateCreated)
	}
}

func TestTransition_UnknownStateRejected(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	err = e.Transition("not-a-state", "bad input")
	if !errors.Is(err, execution.ErrUnknownTargetState) {
		t.Fatalf("Transition() error = %v, want ErrUnknownTargetState", err)
	}
}

func TestTransition_TerminalRejectsFurther(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	for _, s := range []execution.State{execution.StateQueued, execution.StateCancelled} {
		if err := e.Transition(s, "test"); err != nil {
			t.Fatalf("Transition(%s) returned error: %v", s, err)
		}
	}

	err = e.Transition(execution.StateQueued, "resurrect")
	if !errors.Is(err, execution.ErrTerminalState) {
		t.Fatalf("Transition() error = %v, want ErrTerminalState", err)
	}
}

func TestTransition_RecordsHistory(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := e.Transition(execution.StateQueued, "dispatch"); err != nil {
		t.Fatalf("Transition() returned error: %v", err)
	}
	if err := e.Transition(execution.StateRunning, "started"); err != nil {
		t.Fatalf("Transition() returned error: %v", err)
	}

	hist := e.History()
	if len(hist) != 2 {
		t.Fatalf("History() len = %d, want 2", len(hist))
	}
	if hist[0].From != execution.StateCreated || hist[0].To != execution.StateQueued || hist[0].Reason != "dispatch" {
		t.Errorf("hist[0] = %+v, unexpected", hist[0])
	}
	if hist[1].From != execution.StateQueued || hist[1].To != execution.StateRunning {
		t.Errorf("hist[1] = %+v, unexpected", hist[1])
	}
}

func TestStepLifecycle(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := e.StartStep("fetch"); err != nil {
		t.Fatalf("StartStep() returned error: %v", err)
	}
	sr, ok := e.StepRun("fetch")
	if !ok || sr.Status != execution.StepRunning || sr.Attempt != 1 || sr.StartedAt == nil {
		t.Fatalf("StepRun(fetch) after start = %+v, ok=%v", sr, ok)
	}

	if err := e.CompleteStep("fetch"); err != nil {
		t.Fatalf("CompleteStep() returned error: %v", err)
	}
	sr, _ = e.StepRun("fetch")
	if sr.Status != execution.StepCompleted || sr.EndedAt == nil {
		t.Fatalf("StepRun(fetch) after complete = %+v", sr)
	}

	if e.AllStepsCompleted() {
		t.Fatal("AllStepsCompleted() = true with 'summarize' still pending")
	}
	if e.AnyStepFailed() {
		t.Fatal("AnyStepFailed() = true, no step has failed")
	}
}

func TestStepLifecycle_RetryIncrementsAttempt(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := e.StartStep("fetch"); err != nil {
		t.Fatalf("StartStep() returned error: %v", err)
	}
	if err := e.FailStep("fetch", "timeout"); err != nil {
		t.Fatalf("FailStep() returned error: %v", err)
	}

	// A failed step is terminal in this milestone's lightweight model —
	// retrying means the caller starts a *new* StepRun via StartStep,
	// which is rejected once terminal. This documents that boundary:
	// retry orchestration (clearing a step back to pending) is a
	// Scheduler concern, not something this aggregate does implicitly.
	err = e.StartStep("fetch")
	if !errors.Is(err, execution.ErrStepTerminal) {
		t.Fatalf("StartStep() on failed step error = %v, want ErrStepTerminal", err)
	}
}

func TestStepLifecycle_UnknownStep(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := e.StartStep("ghost"); !errors.Is(err, execution.ErrUnknownStep) {
		t.Fatalf("StartStep() error = %v, want ErrUnknownStep", err)
	}
}

func TestAllStepsCompleted(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if e.AllStepsCompleted() {
		t.Fatal("AllStepsCompleted() = true immediately after New()")
	}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	must(e.StartStep("fetch"))
	must(e.CompleteStep("fetch"))
	must(e.StartStep("summarize"))
	must(e.SkipStep("summarize")) // skipped counts as resolved

	if !e.AllStepsCompleted() {
		t.Fatal("AllStepsCompleted() = false, want true (completed + skipped)")
	}
}

func TestClone_Independence(t *testing.T) {
	e, err := execution.New("exec-1", testWorkflow(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := e.Transition(execution.StateQueued, "test"); err != nil {
		t.Fatalf("Transition() returned error: %v", err)
	}
	if err := e.StartStep("fetch"); err != nil {
		t.Fatalf("StartStep() returned error: %v", err)
	}

	clone := e.Clone()
	if err := clone.Transition(execution.StateRunning, "on the clone only"); err != nil {
		t.Fatalf("Transition() on clone returned error: %v", err)
	}
	if err := clone.CompleteStep("fetch"); err != nil {
		t.Fatalf("CompleteStep() on clone returned error: %v", err)
	}

	if e.State() != execution.StateQueued {
		t.Errorf("original State() = %s after mutating clone, want unchanged %s", e.State(), execution.StateQueued)
	}
	sr, _ := e.StepRun("fetch")
	if sr.Status != execution.StepRunning {
		t.Errorf("original fetch status = %s after mutating clone, want unchanged running", sr.Status)
	}
}
