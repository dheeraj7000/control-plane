package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/storage"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

func testExecWorkflow(t *testing.T) workflow.Workflow {
	t.Helper()
	wf, err := workflow.New(uniqueID(t, "wf-for-exec"), "Test", 1, []workflow.Step{
		{ID: "fetch", Type: workflow.StepTypeSearch},
	})
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}
	return wf
}

func TestExecutionRepository_CreateGetRoundTrip(t *testing.T) {
	db := testDB(t)
	repo := storage.NewExecutionRepository(db)
	ctx := context.Background()

	e, err := execution.New(uniqueID(t, "exec"), testExecWorkflow(t), execution.WithAgentID("agent-1"))
	if err != nil {
		t.Fatalf("execution.New() returned error: %v", err)
	}
	if err := e.Transition(execution.StateQueued, "test"); err != nil {
		t.Fatal(err)
	}
	if err := e.StartStep("fetch"); err != nil {
		t.Fatal(err)
	}

	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := repo.Get(ctx, e.ID())
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.AgentID() != "agent-1" || got.State() != execution.StateQueued {
		t.Errorf("Get() identity/state = %+v", got)
	}
	sr, ok := got.StepRun("fetch")
	if !ok || sr.Status != execution.StepRunning || sr.Attempt != 1 {
		t.Errorf("Get() fetch step = %+v, ok=%v, want running/attempt=1", sr, ok)
	}
	if len(got.History()) != 1 {
		t.Errorf("Get().History() len = %d, want 1", len(got.History()))
	}
}

func TestExecutionRepository_UpdatePersistsChanges(t *testing.T) {
	db := testDB(t)
	repo := storage.NewExecutionRepository(db)
	ctx := context.Background()

	e, err := execution.New(uniqueID(t, "exec"), testExecWorkflow(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if err := e.Transition(execution.StateQueued, "a"); err != nil {
		t.Fatal(err)
	}
	if err := e.Transition(execution.StateRunning, "b"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, e); err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}

	got, err := repo.Get(ctx, e.ID())
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.State() != execution.StateRunning {
		t.Errorf("Get().State() = %s, want %s", got.State(), execution.StateRunning)
	}
	if len(got.History()) != 2 {
		t.Errorf("Get().History() len = %d, want 2", len(got.History()))
	}
}

func TestExecutionRepository_UpdateMissing(t *testing.T) {
	db := testDB(t)
	repo := storage.NewExecutionRepository(db)
	e, err := execution.New(uniqueID(t, "ghost"), testExecWorkflow(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(context.Background(), e); !errors.Is(err, execution.ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestExecutionRepository_GetMissing(t *testing.T) {
	db := testDB(t)
	repo := storage.NewExecutionRepository(db)
	if _, err := repo.Get(context.Background(), uniqueID(t, "ghost")); !errors.Is(err, execution.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestExecutionRepository_ListFilters(t *testing.T) {
	db := testDB(t)
	repo := storage.NewExecutionRepository(db)
	ctx := context.Background()
	wf := testExecWorkflow(t)

	running, err := execution.New(uniqueID(t, "exec-running"), wf)
	if err != nil {
		t.Fatal(err)
	}
	if err := running.Transition(execution.StateQueued, "t"); err != nil {
		t.Fatal(err)
	}
	if err := running.Transition(execution.StateRunning, "t"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, running); err != nil {
		t.Fatal(err)
	}

	created, err := execution.New(uniqueID(t, "exec-created"), wf)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, execution.ListFilter{WorkflowID: wf.ID(), State: execution.StateRunning})
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID() != running.ID() {
		t.Fatalf("List(state=running) = %v, want just %s", ids2(list), running.ID())
	}

	all, err := repo.List(ctx, execution.ListFilter{WorkflowID: wf.ID()})
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List(workflow_id) len = %d, want 2", len(all))
	}
}

func ids2(list []*execution.Execution) []string {
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.ID()
	}
	return out
}
