package policy_test

import (
	"context"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/policy"
)

// TestNativeEngine_ComposesRealRules exercises all four Milestone 4
// rules together through one Engine, the way a future caller actually
// will — not just each rule in isolation.
func TestNativeEngine_ComposesRealRules(t *testing.T) {
	timeWindow, err := policy.NewTimeWindowRule(9, 17)
	if err != nil {
		t.Fatalf("NewTimeWindowRule() returned error: %v", err)
	}
	engine, err := policy.NewNativeEngine(policy.EffectDeny,
		policy.NewBudgetRule(),
		policy.NewToolAllowlistRule(map[string][]string{"agent-1": {"github.search"}}),
		policy.NewAllowedModelsRule([]string{"claude-sonnet-5"}),
		timeWindow,
	)
	if err != nil {
		t.Fatalf("NewNativeEngine() returned error: %v", err)
	}

	t.Run("everything checks out", func(t *testing.T) {
		d, err := engine.Evaluate(context.Background(), policy.Input{
			AgentID: "agent-1",
			Tool:    "github.search",
			Model:   "claude-sonnet-5",
			Now:     at(12),
		})
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if !d.Allowed() {
			t.Fatalf("Decision = %+v, want allowed", d)
		}
	})

	t.Run("budget veto beats an otherwise-allowed tool call", func(t *testing.T) {
		d, err := engine.Evaluate(context.Background(), policy.Input{
			AgentID:        "agent-1",
			Tool:           "github.search",
			BudgetExceeded: true,
		})
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if d.Allowed() || d.RuleName != "budget-exceeded" {
			t.Fatalf("Decision = %+v, want denied by budget-exceeded", d)
		}
	})

	t.Run("disallowed model denied even with everything else fine", func(t *testing.T) {
		d, err := engine.Evaluate(context.Background(), policy.Input{
			AgentID: "agent-1",
			Tool:    "github.search",
			Model:   "some-other-model",
			Now:     at(12),
		})
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if d.Allowed() || d.RuleName != "allowed-models" {
			t.Fatalf("Decision = %+v, want denied by allowed-models", d)
		}
	})

	t.Run("outside time window denied even with an allowlisted tool", func(t *testing.T) {
		d, err := engine.Evaluate(context.Background(), policy.Input{
			AgentID: "agent-1",
			Tool:    "github.search",
			Now:     at(2),
		})
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if d.Allowed() || d.RuleName != "time-window" {
			t.Fatalf("Decision = %+v, want denied by time-window", d)
		}
	})

	t.Run("decision point none of the rules care about falls to default", func(t *testing.T) {
		d, err := engine.Evaluate(context.Background(), policy.Input{WorkflowID: "wf-1"})
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if d.Allowed() {
			t.Fatalf("Decision = %+v, want denied (default effect)", d)
		}
	})
}

func TestRuleNames(t *testing.T) {
	tw, err := policy.NewTimeWindowRule(0, 1)
	if err != nil {
		t.Fatalf("NewTimeWindowRule() returned error: %v", err)
	}
	tests := []struct {
		rule policy.Rule
		want string
	}{
		{policy.NewBudgetRule(), "budget-exceeded"},
		{policy.NewToolAllowlistRule(nil), "tool-allowlist"},
		{policy.NewAllowedModelsRule(nil), "allowed-models"},
		{tw, "time-window"},
	}
	for _, tt := range tests {
		if got := tt.rule.Name(); got != tt.want {
			t.Errorf("Name() = %q, want %q", got, tt.want)
		}
	}
}
