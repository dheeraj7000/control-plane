package workflow_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/workflow"
)

func linearSteps() []workflow.Step {
	return []workflow.Step{
		{ID: "fetch", Type: workflow.StepTypeSearch},
		{ID: "summarize", Type: workflow.StepTypeSummarize, DependsOn: []string{"fetch"}},
		{ID: "review", Type: workflow.StepTypeReview, DependsOn: []string{"summarize"}},
	}
}

func TestNew_Valid(t *testing.T) {
	wf, err := workflow.New("wf-1", "Research", 1, linearSteps(),
		workflow.WithDescription("fetches and summarizes"),
		workflow.WithMetadata(map[string]string{"team": "platform"}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if wf.ID() != "wf-1" || wf.Name() != "Research" || wf.Version() != 1 {
		t.Fatalf("unexpected identity: id=%s name=%s version=%d", wf.ID(), wf.Name(), wf.Version())
	}
	if wf.Description() != "fetches and summarizes" {
		t.Errorf("Description() = %q", wf.Description())
	}
	if got := wf.Metadata()["team"]; got != "platform" {
		t.Errorf("Metadata()[team] = %q, want platform", got)
	}
	if len(wf.Steps()) != 3 {
		t.Errorf("Steps() len = %d, want 3", len(wf.Steps()))
	}
}

func TestNew_Errors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wfName  string
		version int
		steps   []workflow.Step
		wantErr error
	}{
		{"empty id", "", "n", 1, linearSteps(), workflow.ErrEmptyID},
		{"empty name", "id", "", 1, linearSteps(), workflow.ErrEmptyName},
		{"bad version", "id", "n", 0, linearSteps(), workflow.ErrInvalidVersion},
		{"no steps", "id", "n", 1, nil, workflow.ErrNoSteps},
		{
			"empty step id", "id", "n", 1,
			[]workflow.Step{{ID: "", Type: workflow.StepTypeSearch}},
			workflow.ErrEmptyStepID,
		},
		{
			"duplicate step id", "id", "n", 1,
			[]workflow.Step{
				{ID: "a", Type: workflow.StepTypeSearch},
				{ID: "a", Type: workflow.StepTypeReview},
			},
			workflow.ErrDuplicateStepID,
		},
		{
			"invalid step type", "id", "n", 1,
			[]workflow.Step{{ID: "a", Type: "not-a-real-type"}},
			workflow.ErrInvalidStepType,
		},
		{
			"self dependency", "id", "n", 1,
			[]workflow.Step{{ID: "a", Type: workflow.StepTypeSearch, DependsOn: []string{"a"}}},
			workflow.ErrSelfDependency,
		},
		{
			"unknown dependency", "id", "n", 1,
			[]workflow.Step{{ID: "a", Type: workflow.StepTypeSearch, DependsOn: []string{"ghost"}}},
			workflow.ErrUnknownDependency,
		},
		{
			"cyclic dependency", "id", "n", 1,
			[]workflow.Step{
				{ID: "a", Type: workflow.StepTypeSearch, DependsOn: []string{"b"}},
				{ID: "b", Type: workflow.StepTypeReview, DependsOn: []string{"a"}},
			},
			workflow.ErrCyclicDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := workflow.New(tt.id, tt.wfName, tt.version, tt.steps)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New() error = %v, want wrapping %v", err, tt.wantErr)
			}
		})
	}
}

func TestTopologicalOrder_RespectsDependencies(t *testing.T) {
	wf, err := workflow.New("wf-1", "Research", 1, linearSteps())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	order := wf.TopologicalOrder()
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	if pos["fetch"] >= pos["summarize"] {
		t.Errorf("fetch (%d) should precede summarize (%d)", pos["fetch"], pos["summarize"])
	}
	if pos["summarize"] >= pos["review"] {
		t.Errorf("summarize (%d) should precede review (%d)", pos["summarize"], pos["review"])
	}
}

func TestTopologicalOrder_DiamondDependency(t *testing.T) {
	steps := []workflow.Step{
		{ID: "start", Type: workflow.StepTypeSearch},
		{ID: "left", Type: workflow.StepTypeSummarize, DependsOn: []string{"start"}},
		{ID: "right", Type: workflow.StepTypeReview, DependsOn: []string{"start"}},
		{ID: "join", Type: workflow.StepTypeModelCall, DependsOn: []string{"left", "right"}},
	}
	wf, err := workflow.New("wf-diamond", "Diamond", 1, steps)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	order := wf.TopologicalOrder()
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	if pos["start"] >= pos["left"] || pos["start"] >= pos["right"] {
		t.Errorf("start must precede both branches: order=%v", order)
	}
	if pos["left"] >= pos["join"] || pos["right"] >= pos["join"] {
		t.Errorf("both branches must precede join: order=%v", order)
	}
}

func TestSteps_ReturnsIndependentSlice(t *testing.T) {
	wf, err := workflow.New("wf-1", "Research", 1, linearSteps())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	got := wf.Steps()
	got[0].ID = "mutated"

	again := wf.Steps()
	if again[0].ID == "mutated" {
		t.Fatal("mutating the returned slice affected the Workflow's internal state")
	}
}

func TestRestore_PreservesCreatedAt(t *testing.T) {
	originalTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	wf, err := workflow.Restore("wf-1", "Research", 1, linearSteps(), originalTime)
	if err != nil {
		t.Fatalf("Restore() returned error: %v", err)
	}
	if !wf.CreatedAt().Equal(originalTime) {
		t.Errorf("CreatedAt() = %v, want %v", wf.CreatedAt(), originalTime)
	}
}

func TestRestore_StillValidates(t *testing.T) {
	cyclic := []workflow.Step{
		{ID: "a", Type: workflow.StepTypeSearch, DependsOn: []string{"b"}},
		{ID: "b", Type: workflow.StepTypeReview, DependsOn: []string{"a"}},
	}
	if _, err := workflow.Restore("wf-1", "n", 1, cyclic, time.Now()); !errors.Is(err, workflow.ErrCyclicDependency) {
		t.Fatalf("Restore() error = %v, want ErrCyclicDependency", err)
	}
}

func TestStep_LookupByID(t *testing.T) {
	wf, err := workflow.New("wf-1", "Research", 1, linearSteps())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	s, ok := wf.Step("summarize")
	if !ok || s.Type != workflow.StepTypeSummarize {
		t.Fatalf("Step(summarize) = (%+v, %v), want the summarize step", s, ok)
	}
	if _, ok := wf.Step("ghost"); ok {
		t.Fatal("Step(ghost) = true, want false for an unknown step id")
	}
}

func TestMetadata_ReturnsIndependentMap(t *testing.T) {
	wf, err := workflow.New("wf-1", "Research", 1, linearSteps(),
		workflow.WithMetadata(map[string]string{"k": "v"}))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	got := wf.Metadata()
	got["k"] = "mutated"

	again := wf.Metadata()
	if again["k"] != "v" {
		t.Fatal("mutating the returned map affected the Workflow's internal state")
	}
}
