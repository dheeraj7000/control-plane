package policy_test

import (
	"context"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/policy"
)

func TestToolAllowlistRule_AllowsListedTool(t *testing.T) {
	r := policy.NewToolAllowlistRule(map[string][]string{"agent-1": {"github.search", "filesystem.read"}})
	d, ok, err := r.Evaluate(context.Background(), policy.Input{AgentID: "agent-1", Tool: "github.search"})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || !d.Allowed() {
		t.Fatalf("Evaluate() = (%+v, %v), want allowed", d, ok)
	}
}

func TestToolAllowlistRule_DeniesUnlistedTool(t *testing.T) {
	r := policy.NewToolAllowlistRule(map[string][]string{"agent-1": {"github.search"}})
	d, ok, err := r.Evaluate(context.Background(), policy.Input{AgentID: "agent-1", Tool: "filesystem.write"})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || d.Allowed() {
		t.Fatalf("Evaluate() = (%+v, %v), want denied", d, ok)
	}
}

func TestToolAllowlistRule_NoOpinionWhenAgentNotConfigured(t *testing.T) {
	r := policy.NewToolAllowlistRule(map[string][]string{"agent-1": {"github.search"}})
	_, ok, err := r.Evaluate(context.Background(), policy.Input{AgentID: "agent-unconfigured", Tool: "github.search"})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if ok {
		t.Fatal("Evaluate() ok = true for an agent with no configured allowlist, want false (no opinion)")
	}
}

func TestToolAllowlistRule_NoOpinionWhenNoToolOrAgent(t *testing.T) {
	r := policy.NewToolAllowlistRule(map[string][]string{"agent-1": {"github.search"}})
	tests := []policy.Input{
		{AgentID: "agent-1", Tool: ""},
		{AgentID: "", Tool: "github.search"},
	}
	for _, in := range tests {
		_, ok, err := r.Evaluate(context.Background(), in)
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if ok {
			t.Errorf("Evaluate(%+v) ok = true, want false", in)
		}
	}
}

func TestToolAllowlistRule_ConfigIsCopied(t *testing.T) {
	original := map[string][]string{"agent-1": {"github.search"}}
	r := policy.NewToolAllowlistRule(original)
	original["agent-1"][0] = "mutated"

	d, ok, err := r.Evaluate(context.Background(), policy.Input{AgentID: "agent-1", Tool: "github.search"})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || !d.Allowed() {
		t.Fatalf("mutating the input map after construction affected the rule's behavior: %+v, ok=%v", d, ok)
	}
}
