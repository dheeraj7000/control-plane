package workflow_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dheeraj7000/control-plane/internal/workflow"
)

func TestUnmarshalJSON_ExampleFile(t *testing.T) {
	// Proves the "canonical format is JSON" decision in workflow.go's
	// package doc against a real, checked-in example rather than just
	// an inline literal — examples/research-workflow.json is meant to
	// double as documentation and as something that actually parses.
	path := filepath.Join("..", "..", "examples", "research-workflow.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var wf workflow.Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}

	if wf.ID() != "wf-research" || wf.Version() != 1 {
		t.Errorf("identity = %s/%d, want wf-research/1", wf.ID(), wf.Version())
	}
	if len(wf.Steps()) != 3 {
		t.Fatalf("Steps() len = %d, want 3", len(wf.Steps()))
	}
	order := wf.TopologicalOrder()
	if order[0] != "fetch" || order[len(order)-1] != "review" {
		t.Errorf("TopologicalOrder() = %v, want fetch first and review last", order)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original, err := workflow.New("wf-1", "Research", 1, linearSteps(),
		workflow.WithDescription("desc"),
		workflow.WithMetadata(map[string]string{"k": "v"}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	var roundTripped workflow.Workflow
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}

	if roundTripped.ID() != original.ID() ||
		roundTripped.Name() != original.Name() ||
		roundTripped.Version() != original.Version() ||
		roundTripped.Description() != original.Description() {
		t.Errorf("round-tripped identity/metadata mismatch: got %+v", roundTripped)
	}
	if len(roundTripped.Steps()) != len(original.Steps()) {
		t.Errorf("round-tripped Steps() len = %d, want %d", len(roundTripped.Steps()), len(original.Steps()))
	}
}

func TestUnmarshalJSON_InvalidWorkflowRejected(t *testing.T) {
	// A cyclic dependency in the JSON must fail the same way it would
	// via workflow.New — UnmarshalJSON is not a side door around validation.
	data := []byte(`{
		"id": "wf-bad",
		"name": "Bad",
		"version": 1,
		"steps": [
			{"id": "a", "type": "search", "depends_on": ["b"]},
			{"id": "b", "type": "review", "depends_on": ["a"]}
		]
	}`)

	var wf workflow.Workflow
	err := json.Unmarshal(data, &wf)
	if !errors.Is(err, workflow.ErrCyclicDependency) {
		t.Fatalf("Unmarshal() error = %v, want ErrCyclicDependency", err)
	}
}
