package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/storage"
	"github.com/dheeraj7000/control-plane/internal/workflow"
)

func TestWorkflowRepository_CreateGetRoundTrip(t *testing.T) {
	db := testDB(t)
	repo := storage.NewWorkflowRepository(db)
	ctx := context.Background()
	id := uniqueID(t, "wf")

	wf, err := workflow.New(id, "Research", 1, []workflow.Step{
		{ID: "fetch", Type: workflow.StepTypeSearch, Config: map[string]any{"query": "x"}},
		{ID: "summarize", Type: workflow.StepTypeSummarize, DependsOn: []string{"fetch"}},
	}, workflow.WithDescription("desc"), workflow.WithMetadata(map[string]string{"team": "platform"}))
	if err != nil {
		t.Fatalf("workflow.New() returned error: %v", err)
	}

	if err := repo.Create(ctx, wf); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := repo.Get(ctx, id, 1)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got.ID() != id || got.Name() != "Research" || got.Description() != "desc" {
		t.Errorf("Get() = %+v, identity/description mismatch", got)
	}
	if got.Metadata()["team"] != "platform" {
		t.Errorf("Get().Metadata()[team] = %q, want platform", got.Metadata()["team"])
	}
	if len(got.Steps()) != 2 {
		t.Fatalf("Get().Steps() len = %d, want 2", len(got.Steps()))
	}
	// Truncate to microseconds before comparing: Postgres TIMESTAMPTZ
	// has microsecond precision, Go's time.Time has nanosecond — a
	// sub-microsecond difference here is expected round-trip precision
	// loss, not evidence CreatedAt wasn't preserved.
	if !got.CreatedAt().Truncate(time.Microsecond).Equal(wf.CreatedAt().Truncate(time.Microsecond)) {
		t.Errorf("Get().CreatedAt() = %v, want %v (preserved, not re-stamped)", got.CreatedAt(), wf.CreatedAt())
	}
	order := got.TopologicalOrder()
	if order[0] != "fetch" || order[1] != "summarize" {
		t.Errorf("Get().TopologicalOrder() = %v, dependency graph not preserved", order)
	}
}

func TestWorkflowRepository_CreateDuplicateRejected(t *testing.T) {
	db := testDB(t)
	repo := storage.NewWorkflowRepository(db)
	ctx := context.Background()
	id := uniqueID(t, "wf")

	wf, err := workflow.New(id, "Test", 1, []workflow.Step{{ID: "a", Type: workflow.StepTypeReview}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, wf); err != nil {
		t.Fatalf("first Create() returned error: %v", err)
	}
	if err := repo.Create(ctx, wf); !errors.Is(err, workflow.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestWorkflowRepository_GetMissing(t *testing.T) {
	db := testDB(t)
	repo := storage.NewWorkflowRepository(db)
	if _, err := repo.Get(context.Background(), uniqueID(t, "ghost"), 1); !errors.Is(err, workflow.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestWorkflowRepository_GetLatest(t *testing.T) {
	db := testDB(t)
	repo := storage.NewWorkflowRepository(db)
	ctx := context.Background()
	id := uniqueID(t, "wf")

	for _, v := range []int{1, 2, 3} {
		wf, err := workflow.New(id, "Test", v, []workflow.Step{{ID: "a", Type: workflow.StepTypeReview}})
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Create(ctx, wf); err != nil {
			t.Fatalf("Create(v=%d) returned error: %v", v, err)
		}
	}

	latest, err := repo.GetLatest(ctx, id)
	if err != nil {
		t.Fatalf("GetLatest() returned error: %v", err)
	}
	if latest.Version() != 3 {
		t.Errorf("GetLatest().Version() = %d, want 3", latest.Version())
	}
}

func TestWorkflowRepository_ListReturnsLatestPerID(t *testing.T) {
	db := testDB(t)
	repo := storage.NewWorkflowRepository(db)
	ctx := context.Background()
	idA, idB := uniqueID(t, "wf-a"), uniqueID(t, "wf-b")

	mustCreate := func(id string, v int) {
		wf, err := workflow.New(id, "Test", v, []workflow.Step{{ID: "a", Type: workflow.StepTypeReview}})
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Create(ctx, wf); err != nil {
			t.Fatalf("Create(%s, v=%d) returned error: %v", id, v, err)
		}
	}
	mustCreate(idA, 1)
	mustCreate(idA, 2)
	mustCreate(idB, 1)

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	byID := map[string]int{}
	for _, wf := range list {
		if wf.ID() == idA || wf.ID() == idB {
			byID[wf.ID()] = wf.Version()
		}
	}
	if byID[idA] != 2 {
		t.Errorf("List()[%s].Version() = %d, want 2 (latest)", idA, byID[idA])
	}
	if byID[idB] != 1 {
		t.Errorf("List()[%s].Version() = %d, want 1", idB, byID[idB])
	}
}
