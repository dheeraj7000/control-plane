package budget_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/budget"
)

func TestInMemoryRepository_GetOrCreate_CreatesOnce(t *testing.T) {
	ctx := context.Background()
	repo := budget.NewInMemoryRepository()

	l1, err := repo.GetOrCreate(ctx, budget.ScopeDaily, "agent-1", "2026-08-07", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatalf("GetOrCreate() returned error: %v", err)
	}
	if err := l1.Charge(budget.Usage{InputTokens: 500}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, l1); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// A second GetOrCreate for the same key must return the existing
	// (charged) ledger, not a fresh zero-usage one.
	l2, err := repo.GetOrCreate(ctx, budget.ScopeDaily, "agent-1", "2026-08-07", budget.Limit{InputTokens: 999999})
	if err != nil {
		t.Fatalf("GetOrCreate() returned error: %v", err)
	}
	if l2.Usage().InputTokens != 500 {
		t.Errorf("second GetOrCreate() Usage().InputTokens = %d, want 500 (existing ledger)", l2.Usage().InputTokens)
	}
	if l2.Limit().InputTokens != 1000 {
		t.Errorf("second GetOrCreate() Limit().InputTokens = %d, want 1000 (original limit, not the new default)", l2.Limit().InputTokens)
	}
}

func TestInMemoryRepository_GetOrCreate_DifferentPeriodsAreIndependent(t *testing.T) {
	ctx := context.Background()
	repo := budget.NewInMemoryRepository()

	today, err := repo.GetOrCreate(ctx, budget.ScopeDaily, "agent-1", "2026-08-07", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err := today.Charge(budget.Usage{InputTokens: 900}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, today); err != nil {
		t.Fatal(err)
	}

	tomorrow, err := repo.GetOrCreate(ctx, budget.ScopeDaily, "agent-1", "2026-08-08", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if tomorrow.Usage().InputTokens != 0 {
		t.Errorf("a new day's ledger has Usage().InputTokens = %d, want 0", tomorrow.Usage().InputTokens)
	}
}

func TestInMemoryRepository_Save_RequiresPriorGetOrCreate(t *testing.T) {
	ctx := context.Background()
	repo := budget.NewInMemoryRepository()

	l, err := budget.New(budget.ScopeExecution, "exec-1", "", budget.Limit{})
	if err != nil {
		t.Fatalf("budget.New() returned error: %v", err)
	}
	if err := repo.Save(ctx, l); !errors.Is(err, budget.ErrNotFound) {
		t.Fatalf("Save() on a never-created ledger error = %v, want ErrNotFound", err)
	}
}

func TestInMemoryRepository_GetOrCreate_IsACopy(t *testing.T) {
	ctx := context.Background()
	repo := budget.NewInMemoryRepository()

	l, err := repo.GetOrCreate(ctx, budget.ScopeExecution, "exec-1", "", budget.Limit{InputTokens: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Charge(budget.Usage{InputTokens: 100}); err != nil {
		t.Fatal(err)
	}
	// Not saved — a fresh GetOrCreate must not see this charge.
	again, err := repo.GetOrCreate(ctx, budget.ScopeExecution, "exec-1", "", budget.Limit{InputTokens: 100})
	if err != nil {
		t.Fatal(err)
	}
	if again.Usage().InputTokens != 0 {
		t.Fatal("GetOrCreate() reflected an unsaved charge made on a previously returned copy")
	}
}
