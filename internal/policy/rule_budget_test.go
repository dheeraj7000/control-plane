package policy_test

import (
	"context"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/policy"
)

func TestBudgetRule_DeniesWhenExceeded(t *testing.T) {
	r := policy.NewBudgetRule()
	d, ok, err := r.Evaluate(context.Background(), policy.Input{BudgetExceeded: true})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok {
		t.Fatal("Evaluate() ok = false, want true when budget exceeded")
	}
	if d.Allowed() {
		t.Fatal("Decision should deny when budget exceeded")
	}
}

func TestBudgetRule_NoOpinionWhenNotExceeded(t *testing.T) {
	r := policy.NewBudgetRule()
	_, ok, err := r.Evaluate(context.Background(), policy.Input{BudgetExceeded: false})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if ok {
		t.Fatal("Evaluate() ok = true, want false when budget is fine")
	}
}
