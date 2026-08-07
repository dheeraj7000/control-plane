package storage_test

import (
	"context"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/budget"
	"github.com/dheeraj7000/control-plane/internal/storage"
)

func TestBudgetRepository_GetOrCreate_CreatesOnceThenReuses(t *testing.T) {
	db := testDB(t)
	repo := storage.NewBudgetRepository(db)
	ctx := context.Background()
	owner := uniqueID(t, "agent")

	l1, err := repo.GetOrCreate(ctx, budget.ScopeDaily, owner, "2026-08-07", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatalf("GetOrCreate() returned error: %v", err)
	}
	if err := l1.Charge(budget.Usage{InputTokens: 500, Cost: 250}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, l1); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	l2, err := repo.GetOrCreate(ctx, budget.ScopeDaily, owner, "2026-08-07", budget.Limit{InputTokens: 999999})
	if err != nil {
		t.Fatalf("second GetOrCreate() returned error: %v", err)
	}
	if l2.Usage().InputTokens != 500 || l2.Usage().Cost != 250 {
		t.Errorf("second GetOrCreate().Usage() = %+v, want persisted 500/250", l2.Usage())
	}
	if l2.Limit().InputTokens != 1000 {
		t.Errorf("second GetOrCreate().Limit().InputTokens = %d, want 1000 (original, not the new default)", l2.Limit().InputTokens)
	}
}

func TestBudgetRepository_DifferentPeriodsAreIndependent(t *testing.T) {
	db := testDB(t)
	repo := storage.NewBudgetRepository(db)
	ctx := context.Background()
	owner := uniqueID(t, "agent")

	today, err := repo.GetOrCreate(ctx, budget.ScopeDaily, owner, "2026-08-07", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if err := today.Charge(budget.Usage{InputTokens: 900}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, today); err != nil {
		t.Fatal(err)
	}

	tomorrow, err := repo.GetOrCreate(ctx, budget.ScopeDaily, owner, "2026-08-08", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if tomorrow.Usage().InputTokens != 0 {
		t.Errorf("a new period's ledger has Usage().InputTokens = %d, want 0", tomorrow.Usage().InputTokens)
	}
}

func TestBudgetRepository_ExceededSurvivesRoundTrip(t *testing.T) {
	db := testDB(t)
	repo := storage.NewBudgetRepository(db)
	ctx := context.Background()
	owner := uniqueID(t, "exec")

	l, err := repo.GetOrCreate(ctx, budget.ScopeExecution, owner, "", budget.Limit{InputTokens: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Charge(budget.Usage{InputTokens: 100}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, l); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetOrCreate(ctx, budget.ScopeExecution, owner, "", budget.Limit{InputTokens: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Exceeded() {
		t.Fatal("Exceeded() = false after round-tripping a ledger charged to its cap")
	}
}
