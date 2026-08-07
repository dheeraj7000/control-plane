package policy_test

import (
	"context"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/policy"
)

func TestAllowedModelsRule_AllowsListedModel(t *testing.T) {
	r := policy.NewAllowedModelsRule([]string{"gpt-5", "claude-sonnet-5"})
	d, ok, err := r.Evaluate(context.Background(), policy.Input{Model: "claude-sonnet-5"})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || !d.Allowed() {
		t.Fatalf("Evaluate() = (%+v, %v), want allowed", d, ok)
	}
}

func TestAllowedModelsRule_DeniesUnlistedModel(t *testing.T) {
	r := policy.NewAllowedModelsRule([]string{"gpt-5"})
	d, ok, err := r.Evaluate(context.Background(), policy.Input{Model: "some-random-model"})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || d.Allowed() {
		t.Fatalf("Evaluate() = (%+v, %v), want denied", d, ok)
	}
}

func TestAllowedModelsRule_NoOpinionWhenModelEmpty(t *testing.T) {
	r := policy.NewAllowedModelsRule([]string{"gpt-5"})
	_, ok, err := r.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if ok {
		t.Fatal("Evaluate() ok = true with no Model set, want false")
	}
}
