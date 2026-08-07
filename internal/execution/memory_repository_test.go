package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/execution"
)

func mustExecution(t *testing.T, id string) *execution.Execution {
	t.Helper()
	e, err := execution.New(id, testWorkflow(t))
	if err != nil {
		t.Fatalf("execution.New() returned error: %v", err)
	}
	return e
}

func TestInMemoryRepository_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := execution.NewInMemoryRepository()
	e := mustExecution(t, "exec-1")

	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	got, err := repo.Get(ctx, "exec-1")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.ID() != "exec-1" {
		t.Errorf("Get().ID() = %s, want exec-1", got.ID())
	}
}

func TestInMemoryRepository_CreateDuplicateRejected(t *testing.T) {
	ctx := context.Background()
	repo := execution.NewInMemoryRepository()
	e := mustExecution(t, "exec-1")

	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("first Create() returned error: %v", err)
	}
	if err := repo.Create(ctx, e); !errors.Is(err, execution.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestInMemoryRepository_GetMissing(t *testing.T) {
	ctx := context.Background()
	repo := execution.NewInMemoryRepository()
	if _, err := repo.Get(ctx, "ghost"); !errors.Is(err, execution.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestInMemoryRepository_UpdateMissing(t *testing.T) {
	ctx := context.Background()
	repo := execution.NewInMemoryRepository()
	e := mustExecution(t, "exec-1")
	if err := repo.Update(ctx, e); !errors.Is(err, execution.ErrNotFound) {
		t.Fatalf("Update() on missing execution error = %v, want ErrNotFound", err)
	}
}

// TestInMemoryRepository_GetIsACopy is the concurrency-story test
// referenced in docs/architecture.md: mutating what Get() returns must
// NOT silently change what's stored. Only Update() persists changes.
// This matches real database semantics and is deliberate — it would be
// easy to make the fake "just work" via shared pointers, which would
// hide bugs that only appear once Milestone 7 swaps in Postgres.
func TestInMemoryRepository_GetIsACopy(t *testing.T) {
	ctx := context.Background()
	repo := execution.NewInMemoryRepository()
	e := mustExecution(t, "exec-1")
	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := repo.Get(ctx, "exec-1")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if err := got.Transition(execution.StateQueued, "mutate the copy"); err != nil {
		t.Fatalf("Transition() on the fetched copy returned error: %v", err)
	}

	again, err := repo.Get(ctx, "exec-1")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if again.State() != execution.StateCreated {
		t.Fatalf("stored State() = %s after mutating an unrelated Get() copy, want unchanged %s",
			again.State(), execution.StateCreated)
	}

	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}
	persisted, err := repo.Get(ctx, "exec-1")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if persisted.State() != execution.StateQueued {
		t.Fatalf("stored State() = %s after Update(), want %s", persisted.State(), execution.StateQueued)
	}
}

func TestInMemoryRepository_ListFilters(t *testing.T) {
	ctx := context.Background()
	repo := execution.NewInMemoryRepository()

	running := mustExecution(t, "exec-running")
	if err := running.Transition(execution.StateQueued, "t"); err != nil {
		t.Fatal(err)
	}
	if err := running.Transition(execution.StateRunning, "t"); err != nil {
		t.Fatal(err)
	}
	created := mustExecution(t, "exec-created")

	for _, e := range []*execution.Execution{running, created} {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("Create() returned error: %v", err)
		}
	}

	list, err := repo.List(ctx, execution.ListFilter{State: execution.StateRunning})
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID() != "exec-running" {
		t.Fatalf("List(State=running) = %v, want just exec-running", ids(list))
	}

	all, err := repo.List(ctx, execution.ListFilter{})
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List(no filter) len = %d, want 2", len(all))
	}
}

func ids(list []*execution.Execution) []string {
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.ID()
	}
	return out
}
