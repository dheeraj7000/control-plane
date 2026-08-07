package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/workflow"
)

func mustWorkflow(t *testing.T, id string, version int) workflow.Workflow {
	t.Helper()
	wf, err := workflow.New(id, "Test", version, linearSteps())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	return wf
}

func TestInMemoryRepository_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := workflow.NewInMemoryRepository()
	wf := mustWorkflow(t, "wf-1", 1)

	if err := repo.Create(ctx, wf); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := repo.Get(ctx, "wf-1", 1)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.ID() != "wf-1" || got.Version() != 1 {
		t.Errorf("Get() = id=%s version=%d, want wf-1/1", got.ID(), got.Version())
	}
}

func TestInMemoryRepository_CreateDuplicateRejected(t *testing.T) {
	ctx := context.Background()
	repo := workflow.NewInMemoryRepository()
	wf := mustWorkflow(t, "wf-1", 1)

	if err := repo.Create(ctx, wf); err != nil {
		t.Fatalf("first Create() returned error: %v", err)
	}
	err := repo.Create(ctx, wf)
	if !errors.Is(err, workflow.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestInMemoryRepository_GetMissing(t *testing.T) {
	ctx := context.Background()
	repo := workflow.NewInMemoryRepository()

	_, err := repo.Get(ctx, "ghost", 1)
	if !errors.Is(err, workflow.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestInMemoryRepository_GetLatest(t *testing.T) {
	ctx := context.Background()
	repo := workflow.NewInMemoryRepository()

	for _, v := range []int{1, 2, 3} {
		if err := repo.Create(ctx, mustWorkflow(t, "wf-1", v)); err != nil {
			t.Fatalf("Create(v=%d) returned error: %v", v, err)
		}
	}

	latest, err := repo.GetLatest(ctx, "wf-1")
	if err != nil {
		t.Fatalf("GetLatest() returned error: %v", err)
	}
	if latest.Version() != 3 {
		t.Errorf("GetLatest().Version() = %d, want 3", latest.Version())
	}
}

func TestInMemoryRepository_List(t *testing.T) {
	ctx := context.Background()
	repo := workflow.NewInMemoryRepository()

	if err := repo.Create(ctx, mustWorkflow(t, "wf-a", 1)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, mustWorkflow(t, "wf-a", 2)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, mustWorkflow(t, "wf-b", 1)); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() returned %d workflows, want 2 (one per id, latest version)", len(list))
	}
	byID := map[string]int{}
	for _, wf := range list {
		byID[wf.ID()] = wf.Version()
	}
	if byID["wf-a"] != 2 {
		t.Errorf("List()[wf-a].Version() = %d, want 2 (latest)", byID["wf-a"])
	}
	if byID["wf-b"] != 1 {
		t.Errorf("List()[wf-b].Version() = %d, want 1", byID["wf-b"])
	}
}
