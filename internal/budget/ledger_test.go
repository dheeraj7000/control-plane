package budget_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/budget"
)

func TestScope_Valid(t *testing.T) {
	for _, s := range []budget.Scope{budget.ScopeExecution, budget.ScopeDaily, budget.ScopeMonthly} {
		if !s.Valid() {
			t.Errorf("Valid(%s) = false, want true", s)
		}
	}
	if budget.Scope("bogus").Valid() {
		t.Error(`Valid("bogus") = true, want false`)
	}
}

func TestPeriodKey(t *testing.T) {
	at := time.Date(2026, 8, 7, 15, 30, 0, 0, time.UTC)
	tests := []struct {
		scope budget.Scope
		want  string
	}{
		{budget.ScopeExecution, ""},
		{budget.ScopeDaily, "2026-08-07"},
		{budget.ScopeMonthly, "2026-08"},
	}
	for _, tt := range tests {
		if got := budget.PeriodKey(tt.scope, at); got != tt.want {
			t.Errorf("PeriodKey(%s, %v) = %q, want %q", tt.scope, at, got, tt.want)
		}
	}
}

func TestNew_Errors(t *testing.T) {
	if _, err := budget.New(budget.ScopeExecution, "", "", budget.Limit{}); !errors.Is(err, budget.ErrEmptyOwnerID) {
		t.Fatalf("New() error = %v, want ErrEmptyOwnerID", err)
	}
	if _, err := budget.New("bogus", "owner", "", budget.Limit{}); !errors.Is(err, budget.ErrInvalidScope) {
		t.Fatalf("New() error = %v, want ErrInvalidScope", err)
	}
}

func TestCharge_AccumulatesUsage(t *testing.T) {
	l, err := budget.New(budget.ScopeExecution, "exec-1", "", budget.Limit{InputTokens: 1000})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := l.Charge(budget.Usage{InputTokens: 100, OutputTokens: 50, Cost: 10}); err != nil {
		t.Fatalf("Charge() returned error: %v", err)
	}
	if err := l.Charge(budget.Usage{InputTokens: 200, OutputTokens: 25, Cost: 5}); err != nil {
		t.Fatalf("Charge() returned error: %v", err)
	}
	got := l.Usage()
	if got.InputTokens != 300 || got.OutputTokens != 75 || got.Cost != 15 {
		t.Errorf("Usage() = %+v, want {300 75 15}", got)
	}
}

func TestCharge_RejectsNegative(t *testing.T) {
	l, err := budget.New(budget.ScopeExecution, "exec-1", "", budget.Limit{})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := l.Charge(budget.Usage{InputTokens: -1}); !errors.Is(err, budget.ErrNegativeCharge) {
		t.Fatalf("Charge() error = %v, want ErrNegativeCharge", err)
	}
}

func TestExceeded_TokensCap(t *testing.T) {
	l, err := budget.New(budget.ScopeExecution, "exec-1", "", budget.Limit{InputTokens: 100})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if l.Exceeded() {
		t.Fatal("Exceeded() = true before any charge")
	}
	if err := l.Charge(budget.Usage{InputTokens: 99}); err != nil {
		t.Fatal(err)
	}
	if l.Exceeded() {
		t.Fatal("Exceeded() = true at 99/100")
	}
	if err := l.Charge(budget.Usage{InputTokens: 1}); err != nil {
		t.Fatal(err)
	}
	if !l.Exceeded() {
		t.Fatal("Exceeded() = false at 100/100, want true (limit reached)")
	}
}

func TestExceeded_CostCap(t *testing.T) {
	l, err := budget.New(budget.ScopeDaily, "agent-1", "2026-08-07", budget.Limit{Cost: 1_000_000}) // $1.00 cap
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := l.Charge(budget.Usage{Cost: 1_500_000}); err != nil { // $1.50 spent
		t.Fatal(err)
	}
	if !l.Exceeded() {
		t.Fatal("Exceeded() = false after spending past the cost cap")
	}
}

func TestExceeded_UnlimitedDimensionNeverExceeds(t *testing.T) {
	l, err := budget.New(budget.ScopeExecution, "exec-1", "", budget.Limit{}) // no caps at all
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := l.Charge(budget.Usage{InputTokens: 1_000_000_000, OutputTokens: 1_000_000_000, Cost: 1_000_000_000}); err != nil {
		t.Fatal(err)
	}
	if l.Exceeded() {
		t.Fatal("Exceeded() = true with no configured caps")
	}
}

func TestClone_Independence(t *testing.T) {
	l, err := budget.New(budget.ScopeExecution, "exec-1", "", budget.Limit{InputTokens: 100})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	clone := l.Clone()
	if err := clone.Charge(budget.Usage{InputTokens: 100}); err != nil {
		t.Fatal(err)
	}
	if l.Usage().InputTokens != 0 {
		t.Errorf("original Usage().InputTokens = %d after charging clone, want 0", l.Usage().InputTokens)
	}
	if !clone.Exceeded() {
		t.Fatal("clone should be exceeded after charging it to its cap")
	}
	if l.Exceeded() {
		t.Fatal("original should be unaffected by charging the clone")
	}
}

func TestCost_USD(t *testing.T) {
	c := budget.Cost(1_500_000)
	if got := c.USD(); got != 1.5 {
		t.Errorf("USD() = %v, want 1.5", got)
	}
}
